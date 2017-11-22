# PostmanLoadTester
This tool written on go lang, help to run postman collections in parallel mode. So you can use it for load testing based on postman collections. 
As a runner it uses newman.

```
npm install -g newman
npm install -g newman-reporter-teamcity
go get
go install
go build
./postman-load-testing -collection <postman_collection_file_or_url> -environment <postman_environment_file_or_url> -i <number_of_iterations> -n <number_of_threads> -d <delay_between_requests_in_miliseconds>
```

### Arguments

```
  -collection string
    	URL or path to a Postman Collection
  -d int
    	Specify the extent of delay between requests (milliseconds) (default 0)
  -environment string
    	Specify a URL or Path to a Postman Environment
  -i int
    	Define the number of iterations to run. (default 1)
  -n int
    	Number of parallel threads (default 1)
```

## Important Note

This projects has very experimental status. Use this at your own risk
