package e2e_test

import (
	"testing"

	"github.com/anand/api-testing-hw/internal/client"
)

func TestGraphQL_HelloQuery(t *testing.T) {
	c := newGraphQLClient(t)

	resp := doJSON(t, c, "POST", "", nil, map[string]string{
		"query": `query Hello {
  hello(person: { name: "Ivan", age: 35 })
}`,
	})
	requireStatus(t, resp, 200)

	var gql client.GraphQLResponse
	decodeJSON(t, resp, &gql)

	if len(gql.Errors) > 0 {
		failWithDiag(t, resp, "GraphQL errors present: %+v", gql.Errors)
	}
	if gql.Data == nil {
		failWithDiag(t, resp, "expected data in GraphQL response")
	}
	hello, ok := gql.Data["hello"].(string)
	if !ok || hello == "" {
		failWithDiag(t, resp, "expected non-empty data.hello string, got %#v", gql.Data["hello"])
	}
	if hello != "Hello Ivan" {
		failWithDiag(t, resp, "data.hello: got %q, want %q", hello, "Hello Ivan")
	}
}

func TestGraphQL_CreatePersonMutation(t *testing.T) {
	c := newGraphQLClient(t)

	const name = "Student"
	const age = 20

	resp := doJSON(t, c, "POST", "", nil, map[string]string{
		"query": `mutation CreatePerson {
  createPerson(person: { name: "Student", age: 20 }) {
    id
    name
    age
  }
}`,
	})
	requireStatus(t, resp, 200)

	var gql client.GraphQLResponse
	decodeJSON(t, resp, &gql)

	if len(gql.Errors) > 0 {
		failWithDiag(t, resp, "GraphQL errors present: %+v", gql.Errors)
	}
	if gql.Data == nil {
		failWithDiag(t, resp, "expected data in GraphQL mutation response")
	}

	rawPerson, ok := gql.Data["createPerson"].(map[string]any)
	if !ok {
		failWithDiag(t, resp, "expected data.createPerson object, got %#v", gql.Data["createPerson"])
	}
	if client.AsString(rawPerson["id"]) == "" {
		failWithDiag(t, resp, "createPerson.id must be present")
	}
	if client.AsString(rawPerson["name"]) != name {
		failWithDiag(t, resp, "createPerson.name: got %q, want %q", client.AsString(rawPerson["name"]), name)
	}
	if client.AsString(rawPerson["age"]) != client.AsString(float64(age)) {
		failWithDiag(t, resp, "createPerson.age: got %q, want %d", client.AsString(rawPerson["age"]), age)
	}
}
