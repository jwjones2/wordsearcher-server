protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    wspb/ws.proto

# mongodb drivers
#go get go.mongodb.org/mongo-driver/mongo