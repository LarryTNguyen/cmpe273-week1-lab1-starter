# Go gRPC Track

```bash
cd go-grpc
go mod tidy
```

## Run Service A

```bash
go run main_a_grpc.go
```

## Run Service B (new terminal)

```bash
go run main_a_grpc.go
```

## Test

```bash
curl "http://127.0.0.1:8081/call-echo?msg=hello"
```

Stop Service A and rerun the curl command to observe failure handling.

##Successful output:

```StatusCode : 200
StatusDescription : OK
Content           : {
                      "service_a": {
                        "echo": "hello"
                      },
                      "service_b": "ok"
                    }
RawContent        : HTTP/1.1 200 OK
                    Content-Length: 65
                    Content-Type: application/json
                    Date: Tue, 03 Feb 2026 05:38:55 GMT

                    {
                      "service_a": {
                        "echo": "hello"
                      },
                      "service_b": "ok"
                    }
Forms             : {}
Headers           : {[Content-Length, 65], [Content-Type, application/json], [Date, Tue, 03 Feb 2026 05:38:55 GMT]}
Images            : {}
InputFields       : {}
Links             : {}
ParsedHtml        : mshtml.HTMLDocumentClass
RawContentLength  : 65
```

## Failure Output

Client side:

```
curl : { "error": "Get \"http://127.0.0.1:8080/echo?msg=hello\": dial tcp 127.0.0.1:8080: connectex: No connection could be made because the target machine
actively refused it.", "message": "failed to reach service A", "service_a": "unavailable", "service_b": "ok", "status": 503 }
At line:1 char:1
+ curl "http://127.0.0.1:8081/call-echo?msg=hello"
+ ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : InvalidOperation: (System.Net.HttpWebRequest:HttpWebRequest) [Invoke-WebRequest], WebException
    + FullyQualifiedErrorId : WebCmdletWebResponseException,Microsoft.PowerShell.Commands.InvokeWebRequestCommand
```

Server side:

```
2026/02/02 21:41:13 service=B endpoint=/call-echo status=error error="Get \"http://127.0.0.1:8080/echo?msg=hello\": dial tcp 127.0.0.1:8080: connectex: No connection could be made because the target machine actively refused it." latency_ms=0
```

This example system is distributed since it has two different processes that each accomplishes different things while communicating through a network. One of the processes (service-b) communicates to the client and takes the request. This process also talks to the second process (service-a) which actually does the required work that the client is asking for. Another aspect that makes this system distributed is the fact that each process works independently and can function when the other is down.
