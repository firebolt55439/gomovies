# gomovies
Golang REST API and locally-hosted web interface to retrieve movie information and recommendations

![Interface](https://github.com/firebolt55439/gomovies/raw/master/assets/Screen%20Shot%202017-09-15%20at%2011.26.57%20PM.png)

##### Currently under heavy development — clearer documentation coming soon.

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
