# gomovies
Golang REST API to retrieve movies information and recommendations

### Installation
```
go get
go build
```

### Use
On one or multiple instances:
```
./gomovies -listen :8081
...
```

On a load balancer instance:
```
./gomovies -listen :8080 -proxy http://localhost:8081,...
```
