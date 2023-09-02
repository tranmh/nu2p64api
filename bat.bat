go build main.go

cd test
newman run nu2p64api.postman_collection.json
