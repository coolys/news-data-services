package main

import (
	"log"
	"net/http"
	"github.com/rs/cors"
	"github.com/rs/rest-layer-mongo"
	"github.com/rs/rest-layer/resource"
	"github.com/rs/rest-layer/rest"
	"github.com/rs/rest-layer/schema"
	"gopkg.in/mgo.v2"
	"golang.org/x/net/context"
)

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

	user = schema.Schema{
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
		},
	}

	// Define a post resource schema
	post = schema.Schema{
		Fields: schema.Fields{
			"id":      schema.IDField,
			"created": schema.CreatedField,
			"updated": schema.UpdatedField,
			"user": {
				Required:   true,
				Filterable: true,
				Validator: &schema.Reference{
					Path: "users",
				},
			},
			"public": {
				Filterable: true,
				Validator:  &schema.Bool{},
			},
			"meta": {
				Schema: &schema.Schema{
					Fields: schema.Fields{
						"title": {
							Required: true,
							Validator: &schema.String{
								MaxLen: 150,
							},
						},
						"body": {
							Validator: &schema.String{
								MaxLen: 100000,
							},
						},
					},
				},
			},
		},
	}
)

type myResponseFormatter struct {
	// Extending default response sender
	rest.DefaultResponseFormatter
}

// Add a wrapper around the list with pagination info
func (r myResponseFormatter) FormatList(ctx context.Context, headers http.Header, l *resource.ItemList, skipBody bool) (context.Context, interface{}) {
	ctx, data := r.DefaultResponseFormatter.FormatList(ctx, headers, l, skipBody)
	return ctx, map[string]interface{}{
		"meta": map[string]int{
			"total": l.Total,
			"page":  l.Page,
		},
		"list": data,
	}
}

func main() {
	session, err := mgo.Dial("127.0.0.1")
	if err != nil {
		log.Fatalf("Can't connect to MongoDB: %s", err)
	}
	db := "test_rest_layer"

	index := resource.NewIndex()

	users := index.Bind("users", user, mongo.NewHandler(session, db, "users"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})

	categories := index.Bind("categories", category, mongo.NewHandler(session, db, "categories"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})
	categories.Bind("parent", "parent", category, mongo.NewHandler(session, db, "categories"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})

	index.Bind("posts", post, mongo.NewHandler(session, db, "posts"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})

	users.Bind("posts", "user", post, mongo.NewHandler(session, db, "posts"), resource.Conf{
		AllowedModes: resource.ReadWrite,
	})

	users.Bind("feeds", "user", post, mongo.NewHandler(session, db, "feeds"), resource.Conf{
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

	api, err := rest.NewHandler(index)
	if err != nil {
		log.Fatalf("Invalid API configuration: %s", err)
	}
	api.ResponseFormatter = &myResponseFormatter{}

	http.Handle("/", cors.New(cors.Options{OptionsPassthrough: true}).Handler(api))

	log.Print("Serving API on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
