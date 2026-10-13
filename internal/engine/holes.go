package engine

import (
	"slices"
	"sort"
)

const (
	holeOfAMessage  = "message "
	holeOfAField    = "field "
	holeOfAProperty = "property "
	holeOfAFunction = "function "
)

type documentedThing struct {
	kind  string
	owner string
	name  string
}

func (d documentedThing) String() string {
	if d.owner == "" {
		return d.kind + d.name
	}
	return d.kind + d.owner + "." + d.name
}

func thingsDocumentedIn(reference *Reference) map[documentedThing]bool {
	things := map[documentedThing]bool{}
	for _, message := range reference.Messages {
		things[documentedThing{kind: holeOfAMessage, name: message}] = true
	}
	for message, fields := range reference.MessageFields {
		for _, field := range fields {
			things[documentedThing{kind: holeOfAField, owner: message, name: field}] = true
		}
	}
	for componentType, properties := range reference.Properties {
		for _, property := range properties {
			things[documentedThing{kind: holeOfAProperty, owner: componentType, name: property}] = true
		}
	}
	for function := range reference.Functions {
		things[documentedThing{kind: holeOfAFunction, name: function}] = true
	}
	return things
}

func FillHoles(oldestFirst []*Reference) {
	documented := make([]map[documentedThing]bool, len(oldestFirst))
	firstSeen, lastSeen := map[documentedThing]int{}, map[documentedThing]int{}
	for index, reference := range oldestFirst {
		documented[index] = thingsDocumentedIn(reference)
		for thing := range documented[index] {
			if _, wasSeen := firstSeen[thing]; !wasSeen {
				firstSeen[thing] = index
			}
			lastSeen[thing] = index
		}
	}
	for thing, first := range firstSeen {
		for index := first + 1; index < lastSeen[thing]; index++ {
			if !documented[index][thing] {
				oldestFirst[index].fill(thing, oldestFirst[first])
			}
		}
	}
	for _, reference := range oldestFirst {
		sort.Strings(reference.Filled)
		sort.Strings(reference.Messages)
	}
}

func (r *Reference) fill(thing documentedThing, whereItWasDocumented *Reference) {
	switch thing.kind {
	case holeOfAMessage:
		r.Messages = append(r.Messages, thing.name)
	case holeOfAField:
		if !slices.Contains(r.Messages, thing.owner) {
			return
		}
		if r.MessageFields == nil {
			r.MessageFields = map[string][]string{}
		}
		r.MessageFields[thing.owner] = append(r.MessageFields[thing.owner], thing.name)
	case holeOfAProperty:
		if _, hasTheKind := r.Properties[thing.owner]; !hasTheKind {
			return
		}
		r.Properties[thing.owner] = append(r.Properties[thing.owner], thing.name)
	case holeOfAFunction:
		r.Functions[thing.name] = whereItWasDocumented.Functions[thing.name]
	}
	r.Filled = append(r.Filled, thing.String())
}
