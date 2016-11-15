

Convert Go func to http.HandleFunc that handle json request and response json




* [To Handler Func](#to-handler-func)
* [Type Response Error](#type-response-error)




## To Handler Func
``` go
func ToHandlerFunc(serverFunc interface{}) http.HandlerFunc
```
ToHandlerFunc convert any go func to a http.HandleFunc,
that will accept json.Unmarshal request body as parameters,
and response with a body with a return values into json.


Very simple types will work
```go
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
```

Or much more complicated types still works
```go
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
```

Or slice, maps, pointers
```go
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
	// [null,{"Error":"require 4 parameters, but only passed in 1 parameters: []interface {}{[]string{\"Felix\"}}","Value":{}}]
	//
	// ["Hi, Mr. Felix, Your zipcode is 100, Your gender is Male",null]
```

errors should expose details in struct
```go
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
```



## Type: Response Error
``` go
type ResponseError struct {
    Error string
    Value interface{}
}
```
The error of the Go func return values will be wrapped with this struct, So that error details can be exposed as json.











