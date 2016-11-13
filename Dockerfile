FROM golang:onbuild

RUN go get "github.com/justinas/alice"
RUN go get "github.com/rs/rest-layer-mem"
RUN go get "github.com/cool-rest/rest-layer/"
RUN go get "github.com/rs/xlog"
RUN go get "github.com/rs/xaccess"
RUN go get "github.com/graphql-go/graphql"

RUN go get "github.com/rs/cors"
RUN go get "github.com/rs/rest-layer-mongo"

EXPOSE 8080
