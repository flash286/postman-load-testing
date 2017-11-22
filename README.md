# PostmanLoadTester
This tool written on go lang, help to run postman collections in parallel mode. So you can use it for load testing based on postman collections. 
As a runner it uses newman.

```
npm install -g newman
npm install -g newman-reporter-teamcity
go install
go build
./postman-load-testing -collection <postman_collection_file_or_url> -environment <postman_environment_file_or_url> -i <number_of_iterations> -n <number_of_threads> -d <delay_between_requests_in_miliseconds>
```


## Important Note

This projects has very experimental status. Use this at your own risk
