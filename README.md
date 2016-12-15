

Convert Go func to http.HandleFunc that handle json request and response json




* [New Status Code Error](#new-status-code-error)
* [To Handler Func](#to-handler-func)
* [Type Response Error](#type-response-error)
* [Type Status Code Error](#type-status-code-error)




## New Status Code Error
``` go
func NewStatusCodeError(code int, innerError error) (err error)
```
NewStatusCodeError for returning an error with http code



## To Handler Func
``` go
func ToHandlerFunc(serverFunc interface{}) http.HandlerFunc
```
ToHandlerFunc convert any go func to a http.HandleFunc,
that will accept json.Unmarshal request body as parameters,
and response with a body with a return values into json.


### 1) Simple types
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

### 2) More complicated types
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

### 3) Slice, maps, pointers
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
	// ["",{"Error":"require 4 parameters, but only passed in 1 parameters: []interface {}{[]string{\"Felix\"}}","Value":{}}]
	//
	// ["Hi, Mr. Felix, Your zipcode is 100, Your gender is Male",null]
```

### 4) First context: If first parameter is a context.Context, It will be passed in with request.Context()
```go
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
```

### 5) Errors handling with details in returned json
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

### 6) Can use get with empty body to fetch the handler
```go
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
```

### 7) Use `NewStatusCodeError` or implement `StatusCodeError` interface to set http status code of response.
```go
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
	// ["",{"Error":"you can't access it","Value":{}}]
```



## Type: Response Error
``` go
type ResponseError struct {
    Error string
    Value interface{}
}
```
ResponseError is error of the Go func return values will be wrapped with this struct, So that error details can be exposed as json.










## Type: Status Code Error
``` go
type StatusCodeError interface {
    StatusCode() int
}
```
StatusCodeError for the error you returned contains a `StatusCode` method, It will be set to to http response.











