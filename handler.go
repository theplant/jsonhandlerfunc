/*
Convert Go func to http.HandleFunc that handle json request and response json
*/
package jsonhandlerfunc

import (
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

	if ft.Kind() != reflect.Func {
		panic("must pass in a func.")
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var params []interface{}
		numIn := ft.NumIn()
		var ptrs = make([]bool, numIn)

		for i := 0; i < numIn; i++ {
			paramType := ft.In(i)

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

		dec := json.NewDecoder(r.Body)
		defer r.Body.Close()

		err := dec.Decode(&params)
		if err != nil {
			returnError(ft, w, fmt.Errorf("%s, func type: %#+v", err, v))
			return
		}

		// log.Printf("params: %#+v\n", params)

		inVals := []reflect.Value{}
		for i, p := range params {
			var val = reflect.ValueOf(p)
			if !ptrs[i] {
				val = reflect.Indirect(val)
			}
			inVals = append(inVals, val)
		}

		if len(params) != numIn {
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

func returnError(ft reflect.Type, w http.ResponseWriter, err error) {
	var errIndex = 0
	errOuts := []interface{}{}
	for i := 0; i < ft.NumOut(); i++ {
		errOuts = append(errOuts, nil)
		if ft.Out(i).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			errIndex = i
		}
	}
	errOuts[errIndex] = &ResponseError{Error: err.Error(), Value: err}
	writeJSONResponse(w, errOuts)
	return
}
