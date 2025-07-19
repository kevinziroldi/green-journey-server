# GreenJourney backend server 

## Description
GreenJourney Server is a RESTful server exposing API endpoints through HTTPS protocol. It is developed in Golang, a language providing high-performance networking and multiprocessing,following MVC design pattern to ease separation of concerns.  
The application was developed for the course Design and Implementation of Mobile Application, at Politecnico di Milano.  
The documentation can be found [here](https://github.com/kevinziroldi/green-journey-server/blob/main/GreenJourneyDD.pdf).

## How to run the server 
You can find the executable file for you OS in the `deliverables` folder.
1. Download the correct executable
2. Open terminal and move to the directory containing the file;
3. Type `./<executable_file_name`, e.g. for MacOS, `./green-journey-server-mac`.

Optionally, you can set the following command line arguments:
* `port` allows to set the port on which the server runs
* `test_mode` can be "real" or "test" and allows to use a different Database for testing 
* `mock_options` can be "true" or "false", allows to generate some fake travel options for demo purposes
* `logger` can be "true" or "false", allows to enable or disable server logs
