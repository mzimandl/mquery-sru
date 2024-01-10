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

package v12

import (
	"encoding/json"
	"fcs/cnf"
	"fcs/corpus"
	"fcs/general"
	"fcs/rdb"
	"fcs/results"
	"fcs/transformers/basic"
	"net/http"
	"strings"
	"text/template"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/gin-gonic/gin"
)

type FCSSubHandlerV12 struct {
	generalConf *cnf.GeneralInfo
	corporaConf *corpus.CorporaSetup
	radapter    *rdb.Adapter
	tmpl        *template.Template

	supportedRecordPackings []string
	supportedOperations     []string

	queryGeneral        []string
	queryExplain        []string
	querySearchRetrieve []string
}

func (a *FCSSubHandlerV12) explain(ctx *gin.Context, fcsResponse *FCSResponse) int {
	// check if all parameters are supported
	for key, _ := range ctx.Request.URL.Query() {
		if !collections.SliceContains(a.queryGeneral, key) && !collections.SliceContains(a.queryExplain, key) {
			fcsResponse.General.Error = &general.FCSError{
				Code:    general.CodeUnsupportedParameter,
				Ident:   key,
				Message: "Unsupported parameter",
			}
			return http.StatusBadRequest
		}
	}

	// prepare response data
	fcsResponse.Explain = FCSExplain{
		ServerName:          a.generalConf.ServerName,
		ServerPort:          a.generalConf.ServerPort,
		Database:            a.generalConf.Database,
		DatabaseTitle:       a.generalConf.DatabaseTitle,
		DatabaseDescription: a.generalConf.DatabaseDescription,
	}
	if ctx.Query("x-fcs-endpoint-description") == "true" {
		for corpusName, _ := range a.corporaConf.Resources {
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

func (a *FCSSubHandlerV12) searchRetrieve(ctx *gin.Context, fcsResponse *FCSResponse) int {
	// check if all parameters are supported
	for key, _ := range ctx.Request.URL.Query() {
		if !collections.SliceContains(a.queryGeneral, key) && !collections.SliceContains(a.querySearchRetrieve, key) {
			fcsResponse.General.Error = &general.FCSError{
				Code:    general.CodeUnsupportedParameter,
				Ident:   key,
				Message: "Unsupported parameter",
			}
			return http.StatusBadRequest
		}
	}

	// prepare query
	fcsQuery := ctx.Query("query")
	if len(fcsQuery) == 0 {
		fcsResponse.General.Error = &general.FCSError{
			Code:    general.CodeMandatoryParameterNotSupplied,
			Ident:   "fcs_query",
			Message: "Mandatory parameter not supplied",
		}
		return http.StatusBadRequest
	}

	transformer, fcsErr := basic.NewBasicTransformer(fcsQuery)
	if fcsErr != nil {
		fcsResponse.General.Error = fcsErr
		return http.StatusInternalServerError
	}

	// get searchable corpora and attrs
	var corpora, searchAttrs []string
	if ctx.Request.URL.Query().Has("x-fcs-context") {
		for _, v := range strings.Split(ctx.Query("x-fcs-context"), ",") {
			resource, ok := a.corporaConf.Resources[v]
			if !ok {
				fcsResponse.General.Error = &general.FCSError{
					Code:    general.CodeUnsupportedParameterValue,
					Ident:   "x-fcs-context",
					Message: "Unknown context " + v,
				}
				return http.StatusBadRequest
			}
			corpora = append(corpora, v)
			searchAttrs = append(searchAttrs, resource.DefaultSearchAttr)
		}
	} else {
		for corpusName, resource := range a.corporaConf.Resources {
			corpora = append(corpora, corpusName)
			searchAttrs = append(searchAttrs, resource.DefaultSearchAttr)
		}
	}

	// make searches
	waits := make([]<-chan *rdb.WorkerResult, len(corpora))
	for i, corpusName := range corpora {
		query, fcsErr := transformer.CreateCQL(searchAttrs[i])
		if fcsErr != nil {
			fcsResponse.General.Error = fcsErr
			return http.StatusInternalServerError
		}
		args, err := json.Marshal(rdb.ConcExampleArgs{
			CorpusPath: a.corporaConf.GetRegistryPath(corpusName),
			QueryLemma: "",
			Query:      query,
			Attrs:      []string{a.corporaConf.Layers.Text, a.corporaConf.Layers.Text}, // twice, so the line parser works TODO
			MaxItems:   10,
		})
		if err != nil {
			fcsResponse.General.Error = &general.FCSError{
				Code:    general.CodeGeneralSystemError,
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
			fcsResponse.General.Error = &general.FCSError{
				Code:    general.CodeGeneralSystemError,
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
			fcsResponse.General.Error = &general.FCSError{
				Code:    general.CodeGeneralSystemError,
				Ident:   err.Error(),
				Message: "General system error",
			}
			return http.StatusInternalServerError
		}
		if err := result.Err(); err != nil {
			fcsResponse.General.Error = &general.FCSError{
				Code:    general.CodeGeneralSystemError,
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

func (a *FCSSubHandlerV12) produceResponse(ctx *gin.Context, fcsResponse *FCSResponse, code int) {
	if err := a.tmpl.ExecuteTemplate(ctx.Writer, "fcs-1.2.xml", fcsResponse); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	ctx.Writer.WriteHeader(code)
}

func (a *FCSSubHandlerV12) Handle(ctx *gin.Context, fcsGeneralResponse general.FCSGeneralResponse) {
	fcsResponse := &FCSResponse{
		General:       fcsGeneralResponse,
		RecordPacking: "xml",
		Operation:     "explain",
	}
	if fcsResponse.General.Error != nil {
		a.produceResponse(ctx, fcsResponse, http.StatusBadRequest)
		return
	}

	recordPacking := ctx.DefaultQuery("recordPacking", fcsResponse.RecordPacking)
	if !collections.SliceContains(a.supportedRecordPackings, recordPacking) {
		fcsGeneralResponse.Error = &general.FCSError{
			Code:    general.CodeUnsupportedRecordPacking,
			Ident:   "recordPacking",
			Message: "Unsupported record packing",
		}
		a.produceResponse(ctx, fcsResponse, http.StatusBadRequest)
		return
	}
	if recordPacking == "xml" {
		ctx.Writer.Header().Set("Content-Type", "application/xml")
	} else if recordPacking == "string" {
		ctx.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	fcsResponse.RecordPacking = recordPacking

	operation := ctx.DefaultQuery("operation", fcsResponse.Operation)
	if !collections.SliceContains(a.supportedOperations, operation) {
		fcsResponse.General.Error = &general.FCSError{
			Code:    general.CodeUnsupportedOperation,
			Ident:   "",
			Message: "Unsupported operation",
		}
		a.produceResponse(ctx, fcsResponse, http.StatusBadRequest)
		return
	}
	fcsResponse.Operation = operation

	code := http.StatusOK
	switch fcsResponse.Operation {
	case "explain":
		code = a.explain(ctx, fcsResponse)
	case "searchRetrieve":
		code = a.searchRetrieve(ctx, fcsResponse)
	}
	a.produceResponse(ctx, fcsResponse, code)
}

func NewFCSSubHandlerV12(
	generalConf *cnf.GeneralInfo,
	corporaConf *corpus.CorporaSetup,
	radapter *rdb.Adapter,
) *FCSSubHandlerV12 {
	return &FCSSubHandlerV12{
		generalConf:             generalConf,
		corporaConf:             corporaConf,
		radapter:                radapter,
		tmpl:                    template.Must(template.New("").Funcs(general.GetTemplateFuncMap()).ParseGlob("handler/v12/templates/*")),
		supportedOperations:     []string{"explain", "scan", "searchRetrieve"},
		supportedRecordPackings: []string{"xml", "string"},
		queryGeneral:            []string{"version", "recordPacking", "operation"},
		queryExplain:            []string{"x-fcs-endpoint-description"},
		querySearchRetrieve:     []string{"query", "x-fcs-context", "x-fcs-dataviews"},
	}
}
