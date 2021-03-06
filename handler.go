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

type Config struct {
	ErrHandler func(oldErr error) (newErr error)
}

var defaultConfig *Config = &Config{}

func (cfg *Config) injectedParams(w http.ResponseWriter, r *http.Request, injectFunc interface{}, ft reflect.Type) (injVals []reflect.Value, shouldReturn bool) {
	if injectFunc == nil {
		return
	}
	v := reflect.ValueOf(injectFunc)
	outVals := v.Call([]reflect.Value{reflect.ValueOf(w), reflect.ValueOf(r)})
	var httpCode int
	var err error
	httpCode, _, injVals, err = cfg.returnVals(outVals)
	if err != nil {
		cfg.returnError(ft, w, err, httpCode)
		shouldReturn = true
	}
	return
}

func contextInjector(w http.ResponseWriter, r *http.Request) (ctx context.Context, err error) {
	ctx = r.Context()
	return
}

type needIndirectValue struct {
	needIndirect bool
	val          interface{}
}

var errorNil = reflect.Zero(reflect.TypeOf((*error)(nil)).Elem())

/*
ToHandlerFunc convert any go func to a http.HandleFunc,
that will accept json.Unmarshal request body as parameters,
and response with a body with a return values into json.

The second argument is an arguments injector, it's parameter should be (w http.ResponseWriter, r *http.Request), and return values
Will be injected to first func's first few arguments.
*/
func ToHandlerFunc(funcs ...interface{}) http.HandlerFunc {
	return defaultConfig.ToHandlerFunc(funcs...)
}

func (cfg *Config) ToHandlerFunc(funcs ...interface{}) http.HandlerFunc {

	if len(funcs) == 0 {
		panic("pass in one or more func, from the second one is all arguments injector.")
	}
	var serverFunc = funcs[0]
	v := reflect.ValueOf(serverFunc)
	ft := v.Type()
	check(ft)
	var firstIsAlsoInjector bool
	var argsInjectors []interface{}
	for i, injector := range funcs {
		injt := reflect.TypeOf(injector)
		if i == 0 {
			if isInjector(injt) {
				firstIsAlsoInjector = true
			} else {
				continue
			}
		}
		check(injt)
		if !isInjector(injt) {
			panic("injector params must be func(w http.ResponseWriter, r *http.Request) ...")
		}
		argsInjectors = append(argsInjectors, injector)
	}
	// if first argument is context, use contextInjector
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	if len(funcs) == 1 && ft.NumIn() > 0 && ft.In(0).Implements(contextType) {
		argsInjectors = append(argsInjectors, contextInjector)
	}

	if !firstIsAlsoInjector {
		checkInjectorsType(ft, argsInjectors)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var injectVals []reflect.Value
		for _, injector := range argsInjectors {
			thisInjectVals, shouldReturn := cfg.injectedParams(w, r, injector, ft)
			if shouldReturn {
				return
			}
			injectVals = append(injectVals, thisInjectVals...)
		}

		if firstIsAlsoInjector {
			injectVals = append(injectVals, errorNil)
			httpCode, outs, _, _ := cfg.returnVals(injectVals)
			w.WriteHeader(httpCode)
			writeJSONResponse(w, outs)
			return
		}

		// log.Printf("injectVals: %#+v\n", len(injectVals))
		injectedCount := len(injectVals)

		var params []interface{}
		var notNilParams []interface{}
		numIn := ft.NumIn()
		var ptrs = make([]bool, numIn)

		for i := 0; i < numIn; i++ {
			if i < injectedCount {
				continue
			}

			paramType := ft.In(i)
			// log.Printf("paramType: %#+v\n", paramType.String())
			ptrs[i] = true
			var pv interface{}
			switch paramType.Kind() {
			case reflect.Chan:
				panic("params can not be chan type.")
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
			notNilParams = append(notNilParams, pv)
		}

		if len(params) > 0 {
			dec := json.NewDecoder(r.Body)
			defer r.Body.Close()
			req := Req{
				Params: &params,
			}
			err := dec.Decode(&req)
			if err != nil {
				log.Println("jsonhandlerfunc: decode request params error:", err)
				cfg.returnError(ft, w, fmt.Errorf("decode request params error"), http.StatusUnprocessableEntity)
				return
			}
		}

		inVals := injectVals
		for i, p := range params {

			var val = reflect.ValueOf(p)
			if !val.IsValid() {
				val = reflect.ValueOf(notNilParams[i])
			}

			if !ptrs[i+injectedCount] {
				val = reflect.Indirect(val)
			}
			inVals = append(inVals, val)
		}

		if len(inVals) != numIn {
			cfg.returnError(ft, w, fmt.Errorf("require %d params, but passed in %d params", numIn, len(inVals)), http.StatusUnprocessableEntity)
			return
		}

		outVals := v.Call(inVals)
		httpCode, outs, _, _ := cfg.returnVals(outVals)
		w.WriteHeader(httpCode)
		writeJSONResponse(w, outs)

		return
	}
}

func (cfg *Config) returnVals(outVals []reflect.Value) (httpCode int, outs []interface{}, normalVals []reflect.Value, err error) {
	normalVals = outVals[0 : len(outVals)-1]
	httpCode = http.StatusOK

	for _, nVal := range normalVals {
		ov := nVal.Interface()
		outs = append(outs, ov)
	}

	last := outVals[len(outVals)-1].Interface()
	if last != nil {
		err = last.(error)
		if httpE, ok := last.(StatusCodeError); ok {
			httpCode = httpE.StatusCode()
		}
		if codeWithErr, ok := last.(*errorWithStatusCode); ok {
			err = codeWithErr.innerErr
		}
		if cfg.ErrHandler != nil {
			err = cfg.ErrHandler(err)
		}
		outs = append(outs, &ResponseError{Error: err.Error(), Value: err})
	} else {
		outs = append(outs, nil)
	}
	return
}

func writeJSONResponse(w http.ResponseWriter, out interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(Resp{Results: out})
	if err != nil {
		log.Printf("writeJSONResponse Write err: %#+v\n", err)
	}
}

type errorWithStatusCode struct {
	HTTPStatusCode int
	innerErr       error
}

func (e *errorWithStatusCode) Error() string {
	return fmt.Sprintf("%d: %s", e.HTTPStatusCode, e.innerErr)
}

func (e *errorWithStatusCode) StatusCode() int {
	return e.HTTPStatusCode
}

// NewStatusCodeError for returning an error with http code
func NewStatusCodeError(code int, innerError error) (err error) {
	err = &errorWithStatusCode{code, innerError}
	return
}

// StatusCodeError for the error you returned contains a `StatusCode` method, It will be set to to http response.
type StatusCodeError interface {
	StatusCode() int
}

/*
ResponseError is error of the Go func return values will be wrapped with this struct, So that error details can be exposed as json.
*/
type ResponseError struct {
	Error string      `json:"error,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

type Req struct {
	Params interface{} `json:"params"`
}

type Resp struct {
	Results interface{} `json:"results"`
}

func checkInjectorsType(ft reflect.Type, injectors []interface{}) {

	var injectedTypes []reflect.Type
	for _, inj := range injectors {
		injt := reflect.TypeOf(inj)
		for i := 0; i < injt.NumOut()-1; i++ {
			injectedTypes = append(injectedTypes, injt.Out(i))
		}
	}

	var argTypes []reflect.Type
	for i := 0; i < ft.NumIn(); i++ {
		if i >= len(injectedTypes) {
			break
		}
		argTypes = append(argTypes, ft.In(i))
	}

	var injectedTypesStr = fmt.Sprintf("%+v", injectedTypes)
	var argTypesStr = fmt.Sprintf("%+v", argTypes)
	if !typesAssignableTo(injectedTypes, argTypes) {
		panic(fmt.Sprintf("%+v params type is %s, but injecting %s", ft, argTypesStr, injectedTypesStr))
	}

}

func typesAssignableTo(toTypes []reflect.Type, fromTypes []reflect.Type) bool {
	if len(toTypes) != len(fromTypes) {
		return false
	}
	if len(toTypes) == 0 {
		return true
	}
	for i, _ := range toTypes {
		if !fromTypes[i].AssignableTo(toTypes[i]) {
			return false
		}
	}
	return true
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

func isInjector(ft reflect.Type) bool {
	expectedTypes := []reflect.Type{
		reflect.TypeOf((*http.ResponseWriter)(nil)).Elem(),
		reflect.TypeOf((*http.Request)(nil)),
	}
	actualTypes := []reflect.Type{}
	for i := 0; i < ft.NumIn(); i++ {
		actualTypes = append(actualTypes, ft.In(i))
	}
	if !typesAssignableTo(actualTypes, expectedTypes) {
		return false
	}
	return true
}

func isError(t reflect.Type) bool {
	return t.Implements(reflect.TypeOf((*error)(nil)).Elem())
}

func (cfg *Config) returnError(ft reflect.Type, w http.ResponseWriter, err error, httpCode int) {
	var errIndex = 0
	errOuts := []interface{}{}
	for i := 0; i < ft.NumOut(); i++ {
		errOuts = append(errOuts, reflect.Zero(ft.Out(i)).Interface())
		if isError(ft.Out(i)) {
			errIndex = i
		}
	}
	if cfg.ErrHandler != nil {
		err = cfg.ErrHandler(err)
	}
	errOuts[errIndex] = &ResponseError{Error: err.Error(), Value: err}
	w.WriteHeader(httpCode)
	writeJSONResponse(w, errOuts)
	return
}
