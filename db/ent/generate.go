package main

import (
	"log"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	err := entc.Generate(
		"./db/ent/schema",
		&gen.Config{
			Target:  "gen/ent",
			Package: "ent",
			Schema:  "ent/schema",
		},
	)
	if err != nil {
		log.Fatal(err)
	}
}