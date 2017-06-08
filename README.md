

Convert Go func to http.HandleFunc that handle json request and response json




* [New Status Code Error](#new-status-code-error)
* [To Handler Func](#to-handler-func)
* [Type Config](#type-config)
  * [To Handler Func](#config-to-handler-func)
* [Type Req](#type-req)
* [Type Resp](#type-resp)
* [Type Response Error](#type-response-error)
* [Type Status Code Error](#type-status-code-error)




## New Status Code Error
``` go
func NewStatusCodeError(code int, innerError error) (err error)
```
NewStatusCodeError for returning an error with http code



## To Handler Func
``` go
func ToHandlerFunc(funcs ...interface{}) http.HandlerFunc
```
ToHandlerFunc convert any go func to a http.HandleFunc,
that will accept json.Unmarshal request body as parameters,
and response with a body with a return values into json.

The second argument is an arguments injector, it's parameter should be (w http.ResponseWriter, r *http.Request), and return values
Will be injected to first func's first few arguments.


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
	// {"results":["",{"error":"Sorry, I don't know about your gender.","value":{}}]}
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
	
	//Output:
	// {"results":["",{"error":"require 4 parameters, but passed in 1 parameters: []interface {}{[]string{\"Felix\"}}","value":{}}]}
	//
	// {"results":["Hi, Mr. Felix, Your zipcode is 100, Your gender is Male",null]}
```

### 4) First context: If first parameter is a context.Context, It will be passed in with request.Context()
```go
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
```

### 5) Errors handling with details in returned json
```go
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
	// {"results":["",{"error":"It crashed.","value":{"ErrorCode":8800,"ErrorDeepReason":"It crashed."}}]}
```

### 6) Can use get with empty body to fetch the handler
```go
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
```

### 7) Use `NewStatusCodeError` or implement `StatusCodeError` interface to set http status code of response.
```go
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
	// {"results":["",{"error":"you can't access it","value":{}}]}
```

### 8) Pass in another injector func to get arguments from *http.Request and pass it to first func.
the argument injector parameters should be `func(w http.ResponseWriter, r *http.Request)`
the return values except the last error will be passed to the first func.
```go
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
	// {"results":["",{"error":"you can't access it","value":{}}]}
	//
	// 200
	// {"results":["cardId: 30, userId: 300, name: Gates, gender: 2",null]}
	//
	// 200
	// {"results":[30,null]}
```

### 9) panic if injectors type not match
```go
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
```

### 10) Config ErrHandler
```go
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
	// {"results":["",{"error":"system error","value":{}}]}
```



## Type: Config
``` go
type Config struct {
    ErrHandler func(oldErr error) (newErr error)
}
```









### Config: To Handler Func
``` go
func (cfg *Config) ToHandlerFunc(funcs ...interface{}) http.HandlerFunc
```



## Type: Req
``` go
type Req struct {
    Params interface{} `json:"params"`
}
```









## Type: Resp
``` go
type Resp struct {
    Results interface{} `json:"results"`
}
```









## Type: Response Error
``` go
type ResponseError struct {
    Error string      `json:"error,omitempty"`
    Value interface{} `json:"value,omitempty"`
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











