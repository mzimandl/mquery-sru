// Copyright 2023 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of MQUERY.
//
//  MQUERY is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  MQUERY is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with MQUERY.  If not, see <https://www.gnu.org/licenses/>.

package handler

import (
	"encoding/json"
	"fcs/corpus"
	"fcs/rdb"
	"fcs/results"
	"net/http"
	"strings"
	"text/template"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/gin-gonic/gin"
)

type Actions struct {
	conf     *corpus.CorporaSetup
	radapter *rdb.Adapter
	tmpl     *template.Template

	supportedRecordPackings []string
	supportedOperations     []string
	supportedVersions       []string

	queryGeneral        []string
	queryExplain        []string
	querySearchRetrieve []string
}

type FCSResourceInfo struct {
	PID         string
	Title       string
	Description string
	URI         string
	Languages   []string
}

type FCSSearchRow struct {
	Position int
	PID      string
	Left     string
	KWIC     string
	Right    string
	Web      string
	Ref      string
}

type FCSExplain struct {
	ServerName          string
	ServerPort          string
	Database            string
	DatabaseTitle       string
	DatabaseDescription string
}

type FCSSearchRetrieve struct {
	Results []FCSSearchRow
}

type FCSResponse struct {
	Version       string
	RecordPacking string
	Operation     string

	MaximumRecords int
	MaximumTerms   int

	Explain        FCSExplain
	Resources      []FCSResourceInfo
	SearchRetrieve FCSSearchRetrieve
	Error          *FCSError
}

func (a *Actions) explain(ctx *gin.Context, fcsResponse *FCSResponse) int {
	// check if all parameters are supported
	for key, _ := range ctx.Request.URL.Query() {
		if !collections.SliceContains(a.queryGeneral, key) && !collections.SliceContains(a.queryExplain, key) {
			fcsResponse.Error = &FCSError{
				Code:    CodeUnsupportedParameter,
				Ident:   key,
				Message: "Unsupported parameter",
			}
			return http.StatusBadRequest
		}
	}

	// prepare response data
	fcsResponse.Explain = FCSExplain{
		ServerName:          ctx.Request.URL.Host,   // TODO
		ServerPort:          ctx.Request.URL.Port(), // TODO
		Database:            ctx.Request.URL.Path,   // TODO
		DatabaseTitle:       "TODO",
		DatabaseDescription: "TODO",
	}
	if ctx.Query("x-fcs-endpoint-description") == "true" {
		for corpusName, _ := range a.conf.Resources {
			fcsResponse.Resources = append(
				fcsResponse.Resources,
				FCSResourceInfo{
					PID:         corpusName,
					Title:       corpusName,
					Description: "TODO",
					URI:         "TODO",
					Languages:   []string{"cs", "TODO"},
				},
			)
		}
	}
	return http.StatusOK
}

func (a *Actions) searchRetrieve(ctx *gin.Context, fcsResponse *FCSResponse) int {
	// check if all parameters are supported
	for key, _ := range ctx.Request.URL.Query() {
		if !collections.SliceContains(a.queryGeneral, key) && !collections.SliceContains(a.querySearchRetrieve, key) {
			fcsResponse.Error = &FCSError{
				Code:    CodeUnsupportedParameter,
				Ident:   key,
				Message: "Unsupported parameter",
			}
			return http.StatusBadRequest
		}
	}

	// prepare query
	fcsQuery := ctx.Query("query")
	if len(fcsQuery) == 0 {
		fcsResponse.Error = &FCSError{
			Code:    CodeMandatoryParameterNotSupplied,
			Ident:   "fcs_query",
			Message: "Mandatory parameter not supplied",
		}
		return http.StatusBadRequest
	}
	query, err := transformFCSQuery(fcsQuery)
	if err != nil {
		fcsResponse.Error = &FCSError{
			Code:    CodeGeneralSystemError,
			Ident:   err.Error(),
			Message: "General system error",
		}
		return http.StatusInternalServerError
	}

	// get searchable corpora
	corpora := make([]string, 0, len(a.conf.Resources))
	for corpusName, _ := range a.conf.Resources {
		corpora = append(corpora, corpusName)
	}
	if ctx.Request.URL.Query().Has("x-fcs-context") {
		fcsContext := strings.Split(ctx.Query("x-fcs-context"), ",")
		for _, v := range fcsContext {
			if !collections.SliceContains(corpora, v) {
				fcsResponse.Error = &FCSError{
					Code:    CodeUnsupportedParameterValue,
					Ident:   "x-fcs-context",
					Message: "Unknown context " + v,
				}
				return http.StatusBadRequest
			}
		}
		corpora = fcsContext
	}

	// make searches
	waits := make([]<-chan *rdb.WorkerResult, len(corpora))
	for i, corpusName := range corpora {
		args, err := json.Marshal(rdb.ConcExampleArgs{
			CorpusPath:    a.conf.GetRegistryPath(corpusName),
			QueryLemma:    "",
			Query:         query,
			Attrs:         []string{"word", "lemma"}, // TODO configurable
			MaxItems:      10,
			ParentIdxAttr: a.conf.Resources[corpusName].SyntaxParentAttr.Name,
		})
		if err != nil {
			fcsResponse.Error = &FCSError{
				Code:    CodeGeneralSystemError,
				Ident:   err.Error(),
				Message: "General system error",
			}
			return http.StatusInternalServerError
		}
		wait, err := a.radapter.PublishQuery(rdb.Query{
			Func: "concExample",
			Args: args,
		})
		if err != nil {
			fcsResponse.Error = &FCSError{
				Code:    CodeGeneralSystemError,
				Ident:   err.Error(),
				Message: "General system error",
			}
			return http.StatusInternalServerError
		}
		waits[i] = wait
	}

	// gather results
	results := make([]results.ConcExample, len(corpora))
	for i, wait := range waits {
		rawResult := <-wait
		result, err := rdb.DeserializeConcExampleResult(rawResult)
		if err != nil {
			fcsResponse.Error = &FCSError{
				Code:    CodeGeneralSystemError,
				Ident:   err.Error(),
				Message: "General system error",
			}
			return http.StatusInternalServerError
		}
		if err := result.Err(); err != nil {
			fcsResponse.Error = &FCSError{
				Code:    CodeGeneralSystemError,
				Ident:   err.Error(),
				Message: "General system error",
			}
			return http.StatusInternalServerError
		}
		results[i] = result
	}

	// transform results
	fcsResponse.SearchRetrieve.Results = make([]FCSSearchRow, 0, 100)
	for i, r := range results {
		for _, l := range r.Lines {
			var left, kwic, right string
			hit := false
			for _, token := range l.Text {
				if token.Strong {
					hit = true
				}
				if hit {
					if token.Strong {
						kwic += token.Word + " "
					} else {
						right += token.Word + " "
					}
				} else {
					left += token.Word + " "
				}
			}
			fcsResponse.SearchRetrieve.Results = append(
				fcsResponse.SearchRetrieve.Results,
				FCSSearchRow{
					Position: len(fcsResponse.SearchRetrieve.Results) + 1,
					PID:      corpora[i],
					Left:     strings.TrimSpace(left),
					KWIC:     strings.TrimSpace(kwic),
					Right:    strings.TrimSpace(right),
					Web:      "TODO",
					Ref:      "TODO",
				},
			)
		}
	}
	return http.StatusOK
}

func (a *Actions) FCSHandler(ctx *gin.Context) {
	fcsResponse := FCSResponse{
		Version:        "1.2",
		RecordPacking:  "xml",
		Operation:      "explain",
		MaximumRecords: 250,
		MaximumTerms:   100,
	}

	recordPacking := ctx.DefaultQuery("recordPacking", fcsResponse.RecordPacking)
	if !collections.SliceContains(a.supportedRecordPackings, recordPacking) {
		fcsResponse.Error = &FCSError{
			Code:    CodeUnsupportedRecordPacking,
			Ident:   "recordPacking",
			Message: "Unsupported record packing",
		}
		if err := a.tmpl.ExecuteTemplate(ctx.Writer, "fcs-1.2.xml", fcsResponse); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		ctx.Writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if recordPacking == "xml" {
		ctx.Writer.Header().Set("Content-Type", "application/xml")
	} else if recordPacking == "string" {
		ctx.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	fcsResponse.RecordPacking = recordPacking

	version := ctx.DefaultQuery("version", fcsResponse.Version)
	if !collections.SliceContains(a.supportedVersions, version) {
		fcsResponse.Error = &FCSError{
			Code:    CodeUnsupportedVersion,
			Ident:   "1.2",
			Message: "Unsupported version " + version,
		}
		if err := a.tmpl.ExecuteTemplate(ctx.Writer, "fcs-1.2.xml", fcsResponse); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		ctx.Writer.WriteHeader(http.StatusBadRequest)
		return
	}
	fcsResponse.Version = version

	operation := ctx.DefaultQuery("operation", fcsResponse.Operation)
	if !collections.SliceContains(a.supportedOperations, operation) {
		fcsResponse.Error = &FCSError{
			Code:    CodeUnsupportedOperation,
			Ident:   "",
			Message: "Unsupported operation",
		}
		if err := a.tmpl.ExecuteTemplate(ctx.Writer, "fcs-1.2.xml", fcsResponse); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		ctx.Writer.WriteHeader(http.StatusBadRequest)
		return
	}
	fcsResponse.Operation = operation

	code := http.StatusOK
	switch fcsResponse.Operation {
	case "explain":
		code = a.explain(ctx, &fcsResponse)
	case "searchRetrieve":
		code = a.searchRetrieve(ctx, &fcsResponse)
	}

	if err := a.tmpl.ExecuteTemplate(ctx.Writer, "fcs-1.2.xml", fcsResponse); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	ctx.Writer.WriteHeader(code)
}

func NewActions(
	conf *corpus.CorporaSetup,
	radapter *rdb.Adapter,
) *Actions {
	return &Actions{
		conf:                    conf,
		radapter:                radapter,
		tmpl:                    template.Must(template.ParseGlob("templates/*")),
		supportedOperations:     []string{"explain", "scan", "searchRetrieve"},
		supportedVersions:       []string{"1.2", "2.0"},
		supportedRecordPackings: []string{"xml", "string"},
		queryGeneral:            []string{"operation", "version", "recordPacking"},
		queryExplain:            []string{"x-fcs-endpoint-description"},
		querySearchRetrieve:     []string{"query", "x-fcs-context", "x-fcs-dataviews"},
	}
}
