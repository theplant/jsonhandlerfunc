package jsonhandlerfunc_test

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jerrs "github.com/jjeffery/errors"
	perrs "github.com/pkg/errors"
	"github.com/theplant/jsonhandlerfunc"
	"github.com/theplant/testingutils"
)

// ### 1) Simple types
func ExampleToHandlerFunc_01helloworld() {

	var helloworld = func(name string, gender int) (r string, err error) {
		if gender == 1 {
			r = fmt.Sprintf("Hi, Mr. %s", name)
		} else if gender == 2 {
			r = fmt.Sprintf("Hi, Mrs. %s", name)
		} else {
			err = fmt.Errorf("Sorry, I don't know about your gender.")
		}
		return
	}

	hf := jsonhandlerfunc.ToHandlerFunc(helloworld)

	responseBody := httpPostJSON(hf, `
		{"params": [
			"Gates",
			1
		]}
	`)
	fmt.Println(responseBody)
	responseBody = httpPostJSON(hf, `
		{"params": [
			"Gates",
			2
		]}
	`)
	fmt.Println(responseBody)
	responseBody = httpPostJSON(hf, `
		{"params": [
			"Gates",
			3
		]}
	`)
	fmt.Println(responseBody)
	//Output:
	// {"results":["Hi, Mr. Gates",null]}
	//
	// {"results":["Hi, Mrs. Gates",null]}
	//
	// {"results":["",{"error":"Sorry, I don't know about your gender."}]}
}

// ### 2) More complicated types
func ExampleToHandlerFunc_02plainstruct() {

	var helloworld = func(name string, p struct {
		Name    string
		Address struct {
			Zipcode  int
			Address1 string
		}
	}) (r string, err error) {
		r = fmt.Sprintf("Hi, Mr. %s, Your zipcode is %d", name, p.Address.Zipcode)
		return
	}

	hf := jsonhandlerfunc.ToHandlerFunc(helloworld)

	responseBody := httpPostJSON(hf, `
		{"params": [
			"Felix",
			{
				"Address": {
					"Zipcode": 100
				}
			}
		]}
	`)
	fmt.Println(responseBody)

	//Output:
	// {"results":["Hi, Mr. Felix, Your zipcode is 100",null]}
}

// ### 3) Slice, maps, pointers
func ExampleToHandlerFunc_03slicemapspointers() {

	var helloworld = func(
		names []string,
		genderOfNames map[string]string,
		p *struct {
			Names   []string
			Address struct {
				Zipcode  int
				Address1 string
			}
		},
		pointerNames *[]string,
	) (r string, err error) {
		r = fmt.Sprintf("Hi, Mr. %s, Your zipcode is %d, Your gender is %s", names[0], p.Address.Zipcode, genderOfNames[names[0]])
		return
	}

	hf := jsonhandlerfunc.ToHandlerFunc(helloworld)

	responseBody := httpPostJSON(hf, `{"params":[ ["Felix"] ]}`)
	fmt.Println(responseBody)

	responseBody = httpPostJSON(hf, `
		{"params": [
			["Felix", "Gates"],
			{
				"Felix": "Male",
				"Gates": "Male"
			},
			{
				"Names": ["F1", "F2"],
				"Address": {
					"Zipcode": 100
				}
			},
			["p1", "p2"]
		]}
	`)
	fmt.Println(responseBody)
	responseBody = httpPostJSON(hf, ``)
	fmt.Println(responseBody)
	//Output:
	// {"results":["",{"error":"require 4 params, but passed in 1 params"}]}
	//
	// {"results":["Hi, Mr. Felix, Your zipcode is 100, Your gender is Male",null]}
	//
	// {"results":["",{"error":"decode request params error"}]}
}

// ### 4) First context: If first parameter is a context.Context, It will be passed in with request.Context()
func ExampleToHandlerFunc_04requestcontext() {
	var helloworld = func(ctx context.Context, name string) (r string, err error) {
		userid := ctx.Value("userid").(string)
		r = fmt.Sprintf("Hello %s, My user id is %s", name, userid)
		return
	}

	hf := jsonhandlerfunc.ToHandlerFunc(helloworld)

	middleware := func(inner http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), "userid", "123"))
			inner(w, r)
		}
	}

	responseBody := httpPostJSON(middleware(hf), `{"params": [ "Hello" ]}`)
	fmt.Println(responseBody)
	//Output:
	// {"results":["Hello Hello, My user id is 123",null]}
}

type complicatedError struct {
	ErrorCode       int
	ErrorDeepReason string
}

func (ce *complicatedError) Error() string {
	return ce.ErrorDeepReason
}

// ### 5) Errors handling with details in returned json
func ExampleToHandlerFunc_05errors() {

	var helloworld = func(name string, gender int) (r string, err error) {
		err = &complicatedError{ErrorCode: 8800, ErrorDeepReason: "It crashed."}
		return
	}

	hf := jsonhandlerfunc.ToHandlerFunc(helloworld)

	responseBody := httpPostJSON(hf, `
		{"params": [
			"Gates",
			1
		]}
	`)
	fmt.Println(responseBody)

	//Output:
	// {"results":["",{"error":"It crashed."}]}
}

// ### 6) Can use get with empty body to fetch the handler
func ExampleToHandlerFunc_06getwithemptybody() {

	var helloworld = func(ctx context.Context) (r string, err error) {
		r = "Done"
		return
	}

	hf := jsonhandlerfunc.ToHandlerFunc(helloworld)

	ts := httptest.NewServer(hf)
	defer ts.Close()
	res, err := http.Get(ts.URL)

	if err != nil {
		log.Fatal(err)
	}
	b, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()
	fmt.Println(string(b))
	//Output:
	// {"results":["Done",null]}
}

// ### 7) Use `NewStatusCodeError` or implement `StatusCodeError` interface to set http status code of response.
func ExampleToHandlerFunc_07httpcode() {

	var helloworld = func(name string, gender int) (r string, err error) {
		err = jsonhandlerfunc.NewStatusCodeError(http.StatusForbidden, fmt.Errorf("you can't access it"))
		return
	}

	hf := jsonhandlerfunc.ToHandlerFunc(helloworld)

	responseBody, code := httpPostJSONReturnCode(hf, `
		{"params": [
			"Gates",
			1
		]}
	`)
	fmt.Println(code)
	fmt.Println(responseBody)
	//Output:
	// 403
	// {"results":["",{"error":"you can't access it"}]}
}

// ### 8) Pass in another injector func to get arguments from *http.Request and pass it to first func.
// the argument injector parameters should be `func(w http.ResponseWriter, r *http.Request)`
// the return values except the last error will be passed to the first func.
func ExampleToHandlerFunc_08argumentsinjector() {
	var helloworld = func(cartId int, userId string, name string, gender int) (r string, err error) {
		r = fmt.Sprintf("cardId: %d, userId: %s, name: %s, gender: %d", cartId, userId, name, gender)
		return
	}

	var argsInjector = func(w http.ResponseWriter, r *http.Request) (cartId int, userId string, err error) {
		cartId = 20
		userId = "100"
		return
	}

	hf := jsonhandlerfunc.ToHandlerFunc(helloworld, argsInjector)
	responseBody, code := httpPostJSONReturnCode(hf, `
		{"params": [
			"Gates",
			2
		]}
	`)
	fmt.Println(code)
	fmt.Println(responseBody)

	var argsInjectorWithError = func(w http.ResponseWriter, r *http.Request) (cartId int, userId string, err error) {
		err = jsonhandlerfunc.NewStatusCodeError(http.StatusForbidden, fmt.Errorf("you can't access it"))
		return
	}
	hf = jsonhandlerfunc.ToHandlerFunc(helloworld, argsInjectorWithError)
	responseBody, code = httpPostJSONReturnCode(hf, `
		{"params": [
			"Gates",
			2
		]}
	`)
	fmt.Println(code)
	fmt.Println(responseBody)

	// You can pass more injectors to addup provide arguments from beginning.
	var cardItInjector = func(w http.ResponseWriter, r *http.Request) (cartId int, err error) {
		cartId = 30
		return
	}
	var userIdInjecter = func(w http.ResponseWriter, r *http.Request) (userId string, err error) {
		userId = "300"
		return
	}
	hf = jsonhandlerfunc.ToHandlerFunc(helloworld, cardItInjector, userIdInjecter)
	responseBody, code = httpPostJSONReturnCode(hf, `
		{"params": [
			"Gates",
			2
		]}
	`)
	fmt.Println(code)
	fmt.Println(responseBody)

	// You can also pass only one injector without main func
	hf = jsonhandlerfunc.ToHandlerFunc(cardItInjector)
	responseBody, code = httpPostJSONReturnCode(hf, "")
	fmt.Println(code)
	fmt.Println(responseBody)

	//Output:
	// 200
	// {"results":["cardId: 20, userId: 100, name: Gates, gender: 2",null]}
	//
	// 403
	// {"results":["",{"error":"you can't access it"}]}
	//
	// 200
	// {"results":["cardId: 30, userId: 300, name: Gates, gender: 2",null]}
	//
	// 200
	// {"results":[30,null]}
}

// ### 9) panic if injectors type not match
func ExampleToHandlerFunc_09injectortypenotmatch() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()

	var inj = func(w http.ResponseWriter, r *http.Request) (a *http.Request, b float64, c string, err error) {
		return
	}

	var f = func(a, b, c string) (err error) {
		return
	}

	jsonhandlerfunc.ToHandlerFunc(f, inj)
	fmt.Println("DONE")
	//Output:
	// func(string, string, string) error params type is [string string string], but injecting [*http.Request float64 string]
}

func ExampleForPointerAddress_injectorbug() {
	type Address struct {
		Name string
	}

	var inj = func(w http.ResponseWriter, r *http.Request) (a, b, c string, err error) {
		a = "1"
		b = "2"
		c = "3"
		return
	}

	var f = func(a, b, c string, add *Address) (err error) {
		err = errors.New(fmt.Sprintf("error %+v", add.Name))
		return
	}
	hf := jsonhandlerfunc.ToHandlerFunc(f, inj)

	responseBody := httpPostJSON(hf, `
		{"params": [
			{
				"Name": "Felix"
			}
		]}
	`)
	fmt.Println(responseBody)
	responseBody = httpPostJSON(hf, `
		{"params": [
			null
		]}
	`)
	fmt.Println(responseBody)
	//Output:
	//{"results":[{"error":"error Felix"}]}
	//
	//{"results":[{"error":"error "}]}

}

// ### 10) Config ErrHandler
func ExampleToHandlerFunc_10ErrHandler() {
	var confidentialErr = fmt.Errorf("Internal error, contains confidential information, should not exposed")
	var errMapping = map[error]error{
		confidentialErr: errors.New("system error"),
	}

	cfg := &jsonhandlerfunc.Config{
		ErrHandler: func(oldErr error) (newErr error) {
			return errMapping[oldErr]
		},
	}
	var helloworld = func(name string, gender int) (r string, err error) {
		err = confidentialErr
		return
	}

	hf := cfg.ToHandlerFunc(helloworld)

	responseBody := httpPostJSON(hf, `
		{"params": [
			"Gates",
			1
		]}
	`)
	fmt.Println(responseBody)
	//Output:
	// {"results":["",{"error":"system error"}]}
}

func httpPostJSON(hf http.HandlerFunc, req string) (r string) {
	r, _ = httpPostJSONReturnCode(hf, req)
	return
}

func httpPostJSONReturnCode(hf http.HandlerFunc, req string) (r string, code int) {
	ts := httptest.NewServer(hf)
	defer ts.Close()
	res, err := http.Post(ts.URL, "application/json", strings.NewReader(req))

	if err != nil {
		panic(err)
	}
	code = res.StatusCode
	b, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()
	r = string(b)
	return
}

func TestJSONEncodeError(t *testing.T) {
	var helloworld = func(name string, gender int) (r string, err error) {
		err = errors.New("hi")
		err = perrs.WithStack(jerrs.With("a", "b").Wrap(err, "hi"))
		return
	}

	hf := jsonhandlerfunc.ToHandlerFunc(helloworld)

	responseBody := httpPostJSON(hf, `
		{"params": [
			"Gates",
			1
		]}
	`)

	diff := testingutils.PrettyJsonDiff(`{"results":["",{"error":"hi a=b: hi"}]}`, responseBody)
	if len(diff) > 0 {
		t.Error(diff)
	}
}
