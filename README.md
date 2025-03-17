                              Local Deployment Guide

Get the GO back-end source code from below repository.

_**https://github.com/wmbpbw/web_page_analyzer_go**_

•
Firstly, deploy the Go backend application. It will deploy go-backend, keycloak , and mongodb as well.

•
Execute below command on GO Backend project root location.


**docker-compose up -d**


Then configure keycloak by accessing below url (Before deploying front-end app).


**Keycloak url : http://localhost:8080**


Login to keycloak admin portal (**username : admin, password : admin**)


Create a realm called “**web-analyzer**”


Create a Realm user (any username, email, first name, last name).


Go to credentials tab and provide credentials for new user.( Make sure temporary password option disabled to use the. Otherwise have to provide a new password at first login)


Goto relam clients and import clients using given JSON files. This will create two clients with all the configurations.

Get JSON files from this  repo : **https://github.com/wmbpbw/keyclok-client-data**

Below are the files

▪
**web-analyzer-backend**

▪
**web-analyzer-frontend**


No need to do anything with mongodb. All setup with the deployment. Can access using Mongo compass with below url if necessary.


**mongodb://localhost:27017**

**Then follow REACT front-end README file instructions to deploy REACT app.**
