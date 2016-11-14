package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
	"github.com/dgrijalva/jwt-go"
	"github.com/dgrijalva/jwt-go/request"
	"github.com/cool-rest/alice"
	"github.com/cool-rest/rest-layer/resource"
	"github.com/cool-rest/rest-layer/rest"
	"github.com/cool-rest/rest-layer/schema"
	"github.com/cool-rest/xaccess"
	"github.com/cool-rest/xlog"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"github.com/cool-rest/rest-layer-mongo"
)

// NOTE: this example show how to integrate REST Layer with JWT. No authentication is performed
// in this example. It is assumed that you are using a third party authentication system that
// generates JWT tokens with a user_id claim.

type key int

const userKey key = 0

// NewContextWithUser stores user into context
func NewContextWithUser(ctx context.Context, user *resource.Item) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// UserFromContext retrieves user from context
func UserFromContext(ctx context.Context) (*resource.Item, bool) {
	user, ok := ctx.Value(userKey).(*resource.Item)
	return user, ok
}

func UserFromToken(users *resource.Resource, ctx context.Context, r *http.Request) (*resource.Item, bool) {
	tokenString, err := request.HeaderExtractor{"Authorization"}.ExtractToken(r)
	fmt.Println("tokenString:", tokenString)
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	if token.Valid {
		fmt.Println("You look nice today")
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			fmt.Println(claims["user_id"])
			user, err := users.Get(ctx, r, claims["user_id"])
			if err == nil && user != nil {
				return user, true
			} else {
				return nil, false
			}
		} else {
			fmt.Println(err)
			return nil, false
		}
	} else {
		fmt.Println("Not valid")
		return nil, false
	}

}

// NewJWTHandler parse and validates JWT token if present and store it in the net/context
func NewJWTHandler(users *resource.Resource, jwtKeyFunc jwt.Keyfunc) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := request.ParseFromRequest(r, request.OAuth2Extractor, jwtKeyFunc)
			if err == request.ErrNoTokenInRequest {
				// If no token is found, let REST Layer hooks decide if the resource is public or not
				next.ServeHTTP(w, r)
				return
			}
			if err != nil || !token.Valid {
				// Here you may want to return JSON error
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			claims := token.Claims.(jwt.MapClaims)
			userID, ok := claims["user_id"].(string)
			if !ok || userID == "" {
				// The provided token is malformed, user_id claim is missing
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
			// Lookup the user by its id
			ctx := r.Context()
			user, err := users.Get(ctx, r, userID)
			if user != nil && err == resource.ErrUnauthorized {
				// Ignore unauthorized errors set by ourselves (see AuthResourceHook)
				err = nil
			}
			if err != nil {
				// If user resource storage handler returned an error, respond with an error
				if err == resource.ErrNotFound {
					http.Error(w, "Invalid credential", http.StatusForbidden)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			// Store it into the request's context
			ctx = NewContextWithUser(ctx, user)
			r = r.WithContext(ctx)
			// If xlog is setup, store the user as logger field
			xlog.FromContext(ctx).SetField("user_id", user.ID)
			next.ServeHTTP(w, r)
		})
	}
}

// AuthResourceHook is a resource event handler that protect the resource from unauthorized users
type AuthResourceHook struct {
	UserField string
	users     *resource.Resource
}

// OnFind implements resource.FindEventHandler interface
func (a AuthResourceHook) OnFind(ctx context.Context, r *http.Request, lookup *resource.Lookup, page, perPage int) error {
	// Reject unauthorized users
	fmt.Println("OnFind ctx:", ctx)
	fmt.Println("OnFind r:", r)
	user, found := UserFromToken(a.users, ctx, r)
	if !found {
		return resource.ErrUnauthorized
	}
	// Add a lookup condition to restrict to result on objects owned by this user
	lookup.AddQuery(schema.Query{
		schema.Equal{Field: a.UserField, Value: user.ID},
	})
	return nil
}

// OnGot implements resource.GotEventHandler interface
func (a AuthResourceHook) OnGot(ctx context.Context, r *http.Request, item **resource.Item, err *error) {
	fmt.Println("OnGot ctx:", ctx)
	fmt.Println("OnGot r:", r)
	// Do not override existing errors
	if err != nil {
		return
	}
	// Reject unauthorized users
	user, found := UserFromToken(a.users, ctx, r)
	if !found {
		*err = resource.ErrUnauthorized
		return
	}
	// Check access right
	if u, found := (*item).Payload[a.UserField]; !found || u != user.ID {
		*err = resource.ErrNotFound
	}
	return
}

// OnInsert implements resource.InsertEventHandler interface
func (a AuthResourceHook) OnInsert(ctx context.Context, r *http.Request, items []*resource.Item) error {
	fmt.Println("OnInsert ctx:", ctx)
	fmt.Println("OnInsert r:", r)
	user, found := UserFromToken(a.users, ctx, r)
	if !found {
		return resource.ErrUnauthorized
	}
	// Check access right
	for _, item := range items {
		if u, found := item.Payload[a.UserField]; found {
			if u != user.ID {
				return resource.ErrUnauthorized
			}
		} else {
			// If no user set for the item, set it to current user
			item.Payload[a.UserField] = user.ID
		}
	}
	return nil
}

// OnUpdate implements resource.UpdateEventHandler interface
func (a AuthResourceHook) OnUpdate(ctx context.Context, r *http.Request, item *resource.Item, original *resource.Item) error {
	fmt.Println("OnUpdate ctx:", ctx)
	fmt.Println("OnUpdate r:", r)
	// Reject unauthorized users
	user, found := UserFromToken(a.users, ctx, r)
	if !found {
		return resource.ErrUnauthorized
	}
	// Check access right
	fmt.Println("original.Payload[a.UserField]:", original.Payload[a.UserField])
	fmt.Println("original.Payload[a.UserField]:", original)
	if u, found := original.Payload[a.UserField]; !found || u != user.ID {
		fmt.Println("u:", u)
		fmt.Println("user.ID:", user.ID)
		fmt.Println("found:", found)
		return resource.ErrUnauthorized
	}
	// Ensure user field is not altered
	fmt.Println("item:", item)
	/*
		if u, found := item.Payload[a.UserField]; !found || u != user.ID {
			eturn resource.ErrUnauthorized
		}*/
	return nil
}

// OnDelete implements resource.DeleteEventHandler interface
func (a AuthResourceHook) OnDelete(ctx context.Context, r *http.Request, item *resource.Item) error {
	fmt.Println("OnDelete ctx:", ctx)
	fmt.Println("OnDelete r:", r)
	// Reject unauthorized users
	user, found := UserFromToken(a.users, ctx, r)
	if !found {
		return resource.ErrUnauthorized
	}
	// Check access right
	if item.Payload[a.UserField] != user.ID {
		return resource.ErrUnauthorized
	}
	return nil
}

// OnClear implements resource.ClearEventHandler interface
func (a AuthResourceHook) OnClear(ctx context.Context, r *http.Request, lookup *resource.Lookup) error {
	fmt.Println("OnClear ctx:", ctx)
	fmt.Println("OnClear r:", r)
	// Reject unauthorized users
	user, found := UserFromToken(a.users, ctx, r)
	if !found {
		return resource.ErrUnauthorized
	}
	// Add a lookup condition to restrict to impact of the clear on objects owned by this user
	lookup.AddQuery(schema.Query{
		schema.Equal{Field: a.UserField, Value: user.ID},
	})
	return nil
}

var (
	category = schema.Schema{
		Fields: schema.Fields{
			"id":      schema.IDField,
			"created": schema.CreatedField,
			"updated": schema.UpdatedField,
			"name": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"parent": {
				Sortable:   true,
				Filterable: true,
				Validator: &schema.Reference{
					Path: "categories",
				},
			},
			"slug": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"description": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"covers": schema.Field{
				Validator: &schema.Dict{},
			},
			"lang_data": schema.Field{
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"status": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
		},
	}

	channel = schema.Schema{
		Fields: schema.Fields{
			"route": schema.Field{
				Validator: &schema.Dict{},
			},
			"type": {
				Validator: &schema.String{},
			},
			"fetch_type": {
				Validator: &schema.String{},
			},
			"source": {
				Validator: &schema.String{},
			},
			"source_type": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"action": {
				Validator: &schema.String{},
			},
			"full_action": {
				Validator: &schema.String{},
			},
			"url": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"slug": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},

			"name": {
				Validator: &schema.String{},
			},

			"description": {
				Validator: &schema.String{},
			},
			"page_id": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"account_id": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"channel_id": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"rss_url": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"web_url": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"from_source": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"channel_type": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"status": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"lang": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"original_source": schema.Field{
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"communities": schema.Field{
				Validator: &schema.Dict{},
			},
			"country": schema.Field{
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"category": schema.Field{
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"covers": schema.Field{
				Validator: &schema.Dict{},
			},
			"logos": schema.Field{
				Validator: &schema.Dict{},
			},
			"channel_data": schema.Field{
				Validator: &schema.Dict{},
			},
			"tags": schema.Field{
				Filterable: true,
				Sortable:   true,
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},
			"topics": schema.Field{
				Filterable: true,
				Sortable:   true,
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},
		},
	}

	country = schema.Schema{
		Fields: schema.Fields{
			"id":      schema.IDField,
			"created": schema.CreatedField,
			"updated": schema.UpdatedField,
			"name": {
				Required:   true,
				Filterable: true,
				Sortable:   true,
				Validator: &schema.String{
					MaxLen: 150,
				},
			},
			"code": {
				Required:   true,
				Filterable: true,
				Sortable:   true,
				Validator: &schema.String{
					MaxLen: 150,
				},
			},
			"status": {
				Validator: &schema.String{
					MaxLen: 150,
				},
			},
		},
	}

	data = schema.Schema{
		Fields: schema.Fields{
			"id":      schema.IDField,
			"created": schema.CreatedField,
			"updated": schema.UpdatedField,
			"url": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"feed": {Validator: &schema.Dict{}},
			"route": {
				Validator: &schema.Dict{},
			},
			"channel":  {Validator: &schema.Dict{}},
			"category": {Validator: &schema.Dict{}},
			"country":  {Validator: &schema.Dict{}},
			"tags": schema.Field{
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},
			"topics": schema.Field{
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},
			"owner": {Validator: &schema.Reference{
				Path: "users",
			}},
			"downloadItems": {
				Validator: &schema.Dict{},
			},
			"video": {
				Validator: &schema.Dict{},
			},
			"news": {
				Validator: &schema.Dict{},
			},
			"photo": {
				Validator: &schema.Dict{},
			},
			"place": {
				Validator: &schema.Dict{},
			},
			"product": {
				Validator: &schema.Dict{},
			},
			"movie": {
				Validator: &schema.Dict{},
			},
			"trip": {
				Validator: &schema.Dict{},
			},
			"job": {
				Validator: &schema.Dict{},
			},
			"weather": {
				Validator: &schema.Dict{},
			},
			"music": {
				Validator: &schema.Dict{},
			},
			"book": {
				Validator: &schema.Dict{},
			},
			"flight": {Validator: &schema.Dict{}},
			"tv": {
				Validator: &schema.Dict{},
			},
			"health": {
				Validator: &schema.Dict{},
			},
			"event": {
				Validator: &schema.Dict{},
			},
			"trends": {
				Validator: &schema.Dict{},
			},
			"stars": {
				Validator: &schema.Dict{},
			},
			"funny": {Validator: &schema.Dict{}},

			"things": {
				Validator: &schema.Dict{},
			},
			"og_data": {
				Validator: &schema.Dict{},
			},
			"status": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
		},
	}

	feed = schema.Schema{
		Fields: schema.Fields{
			"id":      schema.IDField,
			"created": schema.CreatedField,
			"updated": schema.UpdatedField,
			"source_created": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"url": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"title": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"slug": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"description": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"content": {
				Validator: &schema.Array{},
			},
			"source_type": {
				Validator: &schema.String{},
			},
			"lang": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"feed_type": {
				Filterable: true,
				Validator:  &schema.String{},
			},
			"type": {
				Filterable: true,
				Validator:  &schema.String{},
			},
			"views":    {Validator: &schema.Integer{}},
			"likes":    {Validator: &schema.Integer{}},
			"shares":   {Validator: &schema.Integer{}},
			"comments": {Validator: &schema.Integer{}},
			"points":   {Validator: &schema.Integer{}},
			"statictis": {
				Validator: &schema.Dict{},
			},
			"covers": {
				Validator: &schema.Dict{},
			},
			"channel": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"category": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"country": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"tags": schema.Field{
				Filterable: true,
				Sortable:   true,
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},
			"topics": schema.Field{
				Filterable: true,
				Sortable:   true,
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},

			"video": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"news": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"photo": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"place": {
				Validator: &schema.Dict{},
			},
			"product": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"movie": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"trip": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"job": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"weather": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"music": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"book": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"flight": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"tv": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"health": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"event": {Validator: &schema.Reference{
				Path: "events",
			}},
			"trends": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"stars": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"funny": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"things": {Validator: &schema.Dict{}},
			"feed_data": {
				Validator: &schema.Dict{},
			},
			"status": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"owner": {
				Filterable: true,
				Validator: &schema.Reference{
					Path: "users",
				},
			},
		},
	}

	news = schema.Schema{
		Fields: schema.Fields{
			"id":      schema.IDField,
			"created": schema.CreatedField,
			"updated": schema.UpdatedField,
			"source_id": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"url": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"title": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"slug": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"description": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"content": {
				Validator: &schema.Array{},
			},
			"source_created": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"lang": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"covers":  {Validator: &schema.Dict{}},
			"files":   {Validator: &schema.Dict{}},
			"channel": {Validator: &schema.Dict{}},
			"tags": schema.Field{
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},
			"topics": schema.Field{
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},
			"category":  {Validator: &schema.Dict{}},
			"country":   {Validator: &schema.Dict{}},
			"owner":     {Validator: &schema.Dict{}},
			"news_data": {Validator: &schema.Dict{}},
			"status": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
		},
	}

	photo = schema.Schema{
		Fields: schema.Fields{
			"id":      schema.IDField,
			"created": schema.CreatedField,
			"updated": schema.UpdatedField,
			"source_id": {
				Filterable: true,
				Validator:  &schema.String{},
			},
			"url": {
				Filterable: true,
				Validator:  &schema.String{},
			},
			"title": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"slug": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"description": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"content": {
				Validator: &schema.Array{},
			},
			"embed": {
				Validator: &schema.Dict{},
			},
			"source_created": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"lang": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"covers":  {Validator: &schema.Dict{}},
			"files":   {Validator: &schema.Dict{}},
			"channel": {Validator: &schema.Dict{}},
			"tags": schema.Field{
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},
			"topics": schema.Field{
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},
			"category": {Validator: &schema.Dict{}},
			"country":  {Validator: &schema.Dict{}},
			"owner":    {Validator: &schema.Dict{}},

			"photo_data": {Validator: &schema.Dict{}},
			"status": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
		},
	}

	video = schema.Schema{
		Fields: schema.Fields{
			"id":      schema.IDField,
			"created": schema.CreatedField,
			"updated": schema.UpdatedField,
			"source_id": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"url": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"title": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"slug": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"description": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"content": {
				Validator: &schema.Array{},
			},
			"embed": {
				Validator: &schema.Dict{},
			},
			"source_created": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"lang": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"duration": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"length": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
			"covers":  {Validator: &schema.Dict{}},
			"files":   {Validator: &schema.Dict{}},
			"channel": {Validator: &schema.Dict{}},
			"tags": schema.Field{
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},
			"topics": schema.Field{
				Validator: &schema.Array{
					ValuesValidator: &schema.String{},
				},
			},
			"category": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.Dict{},
			},
			"country":    {Validator: &schema.Dict{}},
			"owner":      {Validator: &schema.Dict{}},
			"video_data": {Validator: &schema.Dict{}},
			"status": {
				Filterable: true,
				Sortable:   true,
				Validator:  &schema.String{},
			},
		},
	}
	// Define a user resource schema
	user = schema.Schema{
		Fields: schema.Fields{
			"id": {
				Validator: &schema.String{
					MinLen: 2,
					MaxLen: 50,
				},
			},
			"name": {
				Required:   true,
				Filterable: true,
				Validator: &schema.String{
					MaxLen: 150,
				},
			},
			"password": schema.PasswordField,
		},
	}

	// Define a post resource schema
	post = schema.Schema{
		Fields: schema.Fields{
			"id": schema.IDField,
			// Define a user field which references the user owning the post.
			// See bellow, the content of this field is enforced by the fact
			// that posts is a sub-resource of users.
			"user": {
				//Required:   true,
				Filterable: true,
				Validator: &schema.Reference{
					Path: "users",
				},
				/*OnInit: func(ctx context.Context, value interface{}) interface{} {
					// If not set, set the user to currently logged user if any
					fmt.Printf("value: %#v\n", value)
					if value == nil {
						if user, found := UserFromContext(ctx); found {
							println("coucou")
							value = user.ID
						}
					}
					fmt.Printf("value: %#v\n", value)
					return value
				},*/
			},
			"title": {
				Required: true,
				Validator: &schema.String{
					MaxLen: 150,
				},
			},
			"body": {
				Validator: &schema.String{},
			},
		},
	}
)

var (
	jwtSecret = flag.String("jwt-secret", "secret", "The JWT secret passphrase")
)

func main() {
	flag.Parse()

	session, err := mgo.Dial("127.0.0.1")
	if err != nil {
		log.Fatalf("Can't connect to MongoDB: %s", err)
	}
	db := "hellodb"

	// Create a REST API resource index
	index := resource.NewIndex()

	// Bind user on /users
	users := index.Bind("users", user, mongo.NewHandler(session, db, "users"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})

	// Init the db with some users (user registration is not handled by this example)
	secret, _ := schema.Password{}.Validate("secret")
	users.Insert(context.Background(), nil, []*resource.Item{
		{ID: "jack", Updated: time.Now(), ETag: "abcd", Payload: map[string]interface{}{
			"id":       "jack",
			"name":     "Jack Sparrow",
			"password": secret,
		}},
		{ID: "john", Updated: time.Now(), ETag: "efgh", Payload: map[string]interface{}{
			"id":       "john",
			"name":     "John Doe",
			"password": secret,
		}},
	})

	// Bind post on /posts
	posts := index.Bind("posts", post, mongo.NewHandler(session, db, "posts"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})

	index.Bind("categories", category, mongo.NewHandler(session, db, "categories"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})

	index.Bind("data", data, mongo.NewHandler(session, db, "datas"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})
	index.Bind("feed", feed, mongo.NewHandler(session, db, "feeds"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})
	index.Bind("news", news, mongo.NewHandler(session, db, "news"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})
	index.Bind("video", video, mongo.NewHandler(session, db, "videos"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})
	index.Bind("photo", photo, mongo.NewHandler(session, db, "photos"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})
	index.Bind("country", video, mongo.NewHandler(session, db, "countries"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})
	index.Bind("channel", channel, mongo.NewHandler(session, db, "channels"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})

	// Protect resources
	users.Use(AuthResourceHook{UserField: "id", users: users})
	posts.Use(AuthResourceHook{UserField: "user", users: users})

	// Create API HTTP handler for the resource graph
	api, err := rest.NewHandler(index)
	if err != nil {
		log.Fatalf("Invalid API configuration: %s", err)
	}

	// Setup logger
	c := alice.New()
	c.Append(xlog.NewHandler(xlog.Config{}))
	c.Append(xaccess.NewHandler())
	c.Append(xlog.RequestHandler("req"))
	c.Append(xlog.RemoteAddrHandler("ip"))
	c.Append(xlog.UserAgentHandler("ua"))
	c.Append(xlog.RefererHandler("ref"))
	c.Append(xlog.RequestIDHandler("req_id", "Request-Id"))
	resource.LoggerLevel = resource.LogLevelDebug
	resource.Logger = func(ctx context.Context, level resource.LogLevel, msg string, fields map[string]interface{}) {
		xlog.FromContext(ctx).OutputF(xlog.Level(level), 2, msg, fields)
	}
	// Bind the API under /
	http.Handle("/", c.Then(api))

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
