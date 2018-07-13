# GoMovies!

![Main Interface](https://github.com/firebolt55439/gomovies/raw/master/assets/Screen%20Shot%202018-07-13%20at%2012.32.50%20AM.png)
![Movie Modal](https://github.com/firebolt55439/gomovies/raw/master/assets/Screen%20Shot%202018-07-13%20at%2012.34.44%20AM.png)
![Download Manager](https://github.com/firebolt55439/gomovies/raw/master/assets/Screen%20Shot%202018-07-13%20at%2012.33.11%20AM.png)

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
