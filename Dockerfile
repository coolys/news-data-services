FROM golang:onbuild

RUN go get "github.com/cool-rest/alice"
RUN go get "github.com/cool-rest/rest-layer-mem"
RUN go get "github.com/cool-rest/rest-layer/"
RUN go get "github.com/cool-rest/xlog"
RUN go get "github.com/cool-rest/xaccess"
RUN go get "github.com/graphql-go/graphql"

RUN go get "github.com/cool-rest/cors"
RUN go get "github.com/cool-rest/rest-layer-mongo"

EXPOSE 8080
