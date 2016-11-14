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
	//"github.com/cool-rest/rest-layer-mem"
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

func UserFromToken(users *resource.Resource, ctx context.Context, r *http.Request) (*resource.Item, bool){
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
	} else{
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
	users *resource.Resource

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

	// Protect resources
	users.Use(AuthResourceHook{UserField: "id", users:users})
	posts.Use(AuthResourceHook{UserField: "user", users:users})

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

	// Setup auth middleware
	jwtSecretBytes := []byte(*jwtSecret)
	c.Append(NewJWTHandler(users, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrInvalidKey
		}
		return jwtSecretBytes, nil
	}))

	// Bind the API under /
	http.Handle("/", c.Then(api))

	// Demo tokens
	jackToken := jwt.New(jwt.SigningMethodHS256)
	jackClaims := jackToken.Claims.(jwt.MapClaims)
	jackClaims["user_id"] = "jack"
	jackTokenString, err := jackToken.SignedString(jwtSecretBytes)
	if err != nil {
		log.Fatal(err)
	}
	johnToken := jwt.New(jwt.SigningMethodHS256)
	johnClaims := johnToken.Claims.(jwt.MapClaims)
	johnClaims["user_id"] = "john"
	johnTokenString, err := johnToken.SignedString(jwtSecretBytes)
	if err != nil {
		log.Fatal(err)
	}
	// Serve it
	log.Print("Serving API on http://localhost:8080")
	log.Printf("Your token secret is %q, change it with the `-jwt-secret' flag", *jwtSecret)
	log.Print("Play with tokens:\n",
		"\n",
		"- http :8080/posts access_token==", johnTokenString, " title=\"John's post\"\n",
		"- http :8080/posts access_token==", johnTokenString, "\n",
		"- http :8080/posts\n",
		"\n",
		"- http :8080/posts access_token==", jackTokenString, " title=\"Jack's post\"\n",
		"- http :8080/posts access_token==", jackTokenString, "\n",
	)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
