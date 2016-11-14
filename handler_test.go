package jsonhandlerfunc

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
)

// Very simple types will work
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
	// ["Hi, Mrs. Gates",null]
	// ["",{"Error":"Sorry, I don't know about your gender.","Value":{}}]
}

// Or much more complicated types still works
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

// Or slice, maps, pointers
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
	// [null,{"Error":"require 4 parameters, but only passed in 1 parameters: [ [\"Felix\"] ]","Value":{}}]
	// ["Hi, Mr. Felix, Your zipcode is 100, Your gender is Male",null]
}

type complicatedError struct {
	ErrorCode       int
	ErrorDeepReason string
}

func (ce *complicatedError) Error() string {
	return ce.ErrorDeepReason
}

// errors should expose details in struct
func ExampleToHandlerFunc_4errors() {

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

func httpPostJSON(hf http.HandlerFunc, req string) (r string) {
	ts := httptest.NewServer(hf)
	defer ts.Close()
	res, err := http.Post(ts.URL, "application/json", strings.NewReader(req))

	if err != nil {
		log.Fatal(err)
	}
	b, _ := ioutil.ReadAll(res.Body)
	res.Body.Close()
	r = string(b)
	return
}
