package jsonhandlerfunc

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
)

// ### 1) Simple types
func ExampleToHandlerFunc_1helloworld() {

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

	hf := ToHandlerFunc(helloworld)

	responseBody := httpPostJSON(hf, `
		[
			"Gates",
			1
		]
	`)
	fmt.Println(responseBody)
	responseBody = httpPostJSON(hf, `
		[
			"Gates",
			2
		]
	`)
	fmt.Println(responseBody)
	responseBody = httpPostJSON(hf, `
		[
			"Gates",
			3
		]
	`)
	fmt.Println(responseBody)
	//Output:
	// ["Hi, Mr. Gates",null]
	//
	// ["Hi, Mrs. Gates",null]
	//
	// ["",{"Error":"Sorry, I don't know about your gender.","Value":{}}]
}

// ### 2) More complicated types
func ExampleToHandlerFunc_2plainstruct() {

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

	hf := ToHandlerFunc(helloworld)

	responseBody := httpPostJSON(hf, `
		[
			"Felix",
			{
				"Address": {
					"Zipcode": 100
				}
			}
		]
	`)
	fmt.Println(responseBody)

	//Output:
	// ["Hi, Mr. Felix, Your zipcode is 100",null]
}

// ### 3) Slice, maps, pointers
func ExampleToHandlerFunc_3slicemapspointers() {

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

	hf := ToHandlerFunc(helloworld)

	responseBody := httpPostJSON(hf, `[ ["Felix"] ]`)
	fmt.Println(responseBody)

	responseBody = httpPostJSON(hf, `
		[
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
		]
	`)
	fmt.Println(responseBody)

	//Output:
	// ["",{"Error":"require 4 parameters, but only passed in 1 parameters: []interface {}{[]string{\"Felix\"}}","Value":{}}]
	//
	// ["Hi, Mr. Felix, Your zipcode is 100, Your gender is Male",null]
}

// ### 4) First context: If first parameter is a context.Context, It will be passed in with request.Context()
func ExampleToHandlerFunc_4requestcontext() {
	var helloworld = func(ctx context.Context, name string) (r string, err error) {
		userid := ctx.Value("userid").(string)
		r = fmt.Sprintf("Hello %s, My user id is %s", name, userid)
		return
	}

	hf := ToHandlerFunc(helloworld)

	middleware := func(inner http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), "userid", "123"))
			inner(w, r)
		}
	}

	responseBody := httpPostJSON(middleware(hf), `[ "Hello" ]`)
	fmt.Println(responseBody)
	//Output:
	// ["Hello Hello, My user id is 123",null]
}

type complicatedError struct {
	ErrorCode       int
	ErrorDeepReason string
}

func (ce *complicatedError) Error() string {
	return ce.ErrorDeepReason
}

// ### 5) Errors handling with details in returned json
func ExampleToHandlerFunc_5errors() {

	var helloworld = func(name string, gender int) (r string, err error) {
		err = &complicatedError{ErrorCode: 8800, ErrorDeepReason: "It crashed."}
		return
	}

	hf := ToHandlerFunc(helloworld)

	responseBody := httpPostJSON(hf, `
		[
			"Gates",
			1
		]
	`)
	fmt.Println(responseBody)

	//Output:
	// ["",{"Error":"It crashed.","Value":{"ErrorCode":8800,"ErrorDeepReason":"It crashed."}}]
}

// ### 6) Can use get with empty body to fetch the handler
func ExampleToHandlerFunc_6getwithemptybody() {

	var helloworld = func(ctx context.Context) (r string, err error) {
		r = "Done"
		return
	}

	hf := ToHandlerFunc(helloworld)

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
	// ["Done",null]
}

// ### 7) Use `NewStatusCodeError` or implement `StatusCodeError` interface to set http status code of response.
func ExampleToHandlerFunc_7httpcode() {

	var helloworld = func(name string, gender int) (r string, err error) {
		err = NewStatusCodeError(http.StatusForbidden, fmt.Errorf("you can't access it"))
		return
	}

	hf := ToHandlerFunc(helloworld)

	responseBody, code := httpPostJSONReturnCode(hf, `
		[
			"Gates",
			1
		]
	`)
	fmt.Println(code)
	fmt.Println(responseBody)

	//Output:
	// 403
	// ["",{"Error":"403: you can't access it","Value":{"HTTPStatusCode":403}}]
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
		log.Fatal(err)
	}
	code = res.StatusCode
	b, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()
	r = string(b)
	return
}
