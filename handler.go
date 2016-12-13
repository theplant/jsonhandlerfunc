/*
Convert Go func to http.HandleFunc that handle json request and response json
*/
package jsonhandlerfunc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
)

/*
ToHandlerFunc convert any go func to a http.HandleFunc,
that will accept json.Unmarshal request body as parameters,
and response with a body with a return values into json.
*/
func ToHandlerFunc(serverFunc interface{}) http.HandlerFunc {
	v := reflect.ValueOf(serverFunc)
	ft := v.Type()
	check(ft)

	return func(w http.ResponseWriter, r *http.Request) {
		var params []interface{}
		numIn := ft.NumIn()
		var ptrs = make([]bool, numIn)

		contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
		var contextCount = 0

		for i := 0; i < numIn; i++ {
			paramType := ft.In(i)

			if i == 0 && paramType.Implements(contextType) {
				contextCount = 1
				continue
			}

			// log.Printf("paramType: %#+v\n", paramType.String())
			ptrs[i] = true
			var pv interface{}
			switch paramType.Kind() {
			case reflect.Chan:
				panic("parameters can not be chan type.")
			case reflect.Ptr:
				pv = reflect.New(paramType.Elem()).Interface()
			case reflect.Array, reflect.Slice, reflect.Map:
				pv = reflect.New(paramType).Interface()
				ptrs[i] = false
			default:
				pv = reflect.New(paramType).Interface()
				ptrs[i] = false
			}
			// log.Printf("pv: %#+v\n", pv)

			params = append(params, pv)
		}

		if len(params) > 0 {
			dec := json.NewDecoder(r.Body)
			defer r.Body.Close()

			err := dec.Decode(&params)
			if err != nil {
				returnError(ft, w, fmt.Errorf("%s, func type: %#+v", err, v))
				return
			}
		}

		// log.Printf("params: %#+v\n", params)

		inVals := []reflect.Value{}
		if contextCount == 1 {
			inVals = append(inVals, reflect.ValueOf(r.Context()))
		}
		for i, p := range params {
			var val = reflect.ValueOf(p)
			if !ptrs[i] {
				val = reflect.Indirect(val)
			}
			inVals = append(inVals, val)
		}

		if len(params)+contextCount != numIn {
			parsedParams := []interface{}{}
			for _, rv := range inVals {
				parsedParams = append(parsedParams, rv.Interface())
			}
			returnError(ft, w, fmt.Errorf("require %d parameters, but only passed in %d parameters: %#+v", numIn, len(params), parsedParams))
			return
		}

		outVals := v.Call(inVals)
		var outs []interface{}
		for _, outVal := range outVals {
			ov := outVal.Interface()
			if e, ok := ov.(error); ok {
				ov = &ResponseError{Error: e.Error(), Value: e}
			}
			outs = append(outs, ov)
		}
		writeJSONResponse(w, outs)
		return
	}
}

func writeJSONResponse(w http.ResponseWriter, out interface{}) {
	enc := json.NewEncoder(w)
	err := enc.Encode(out)
	if err != nil {
		log.Printf("writeJSONResponse Write err: %#+v\n", err)
	}

}

/*
The error of the Go func return values will be wrapped with this struct, So that error details can be exposed as json.
*/
type ResponseError struct {
	Error string
	Value interface{}
}

func check(ft reflect.Type) {
	if ft.Kind() != reflect.Func {
		panic("must pass in a func.")
	}
	if !isError(ft.Out(ft.NumOut() - 1)) {
		panic("func's last return value must be error.")
	}

	for i := 0; i < ft.NumIn(); i++ {
		if ft.In(i).Kind() == reflect.Chan {
			panic("func arguments can not be chan type.")
		}
	}
	for i := 0; i < ft.NumOut(); i++ {
		if ft.Out(i).Kind() == reflect.Chan {
			panic("func return values can not be chan type.")
		}
	}
}

func isError(t reflect.Type) bool {
	return t.Implements(reflect.TypeOf((*error)(nil)).Elem())
}

func returnError(ft reflect.Type, w http.ResponseWriter, err error) {
	var errIndex = 0
	errOuts := []interface{}{}
	for i := 0; i < ft.NumOut(); i++ {
		errOuts = append(errOuts, reflect.Zero(ft.Out(i)).Interface())
		if isError(ft.Out(i)) {
			errIndex = i
		}
	}
	errOuts[errIndex] = &ResponseError{Error: err.Error(), Value: err}
	writeJSONResponse(w, errOuts)
	return
}
