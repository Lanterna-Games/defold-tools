package project

import "github.com/Lanterna-Games/defold-tools/internal/textproto"

func (p *Project) collectionFrom(filePath, resourcePath string, document *textproto.Message) *Collection {
	collection := &Collection{Path: resourcePath, Name: document.Text("name")}
	for _, field := range document.Fields {
		if field.Message == nil {
			continue
		}
		switch field.Name {
		case "instances":
			instance := instanceFrom(filePath, field)
			instance.Prototype = field.Message.Text("prototype")
			collection.Instances = append(collection.Instances, instance)
		case "embedded_instances":
			instance := instanceFrom(filePath, field)
			instance.EmbeddedObject = p.embeddedGameObjectOf(filePath, resourcePath, field.Message)
			collection.Instances = append(collection.Instances, instance)
		case "collection_instances":
			collection.SubCollections = append(collection.SubCollections, subCollectionFrom(filePath, field))
		}
	}
	return collection
}

func instanceFrom(filePath string, field textproto.Field) Instance {
	instance := Instance{
		ID:       field.Message.Text("id"),
		Location: locationOfID(filePath, field.Message, field.Position),
		IDWhere:  whereTheTextOf(filePath, field.Message, "id"),
	}
	for _, child := range field.Message.Scalars("children") {
		instance.Children = append(instance.Children, Child{
			ID:       child.Text,
			Location: Location{File: filePath, Position: child.Position},
			Where:    Location{File: filePath, Position: child.TextPosition()},
		})
	}
	for _, component := range field.Message.Messages("component_properties") {
		instance.PropertyOverrides = append(instance.PropertyOverrides, propertyOverridesFrom(filePath, component)...)
	}
	return instance
}

func (p *Project) embeddedGameObjectOf(filePath, collectionPath string, instance *textproto.Message) *GameObject {
	data, hasData := instance.Scalar(embeddedDocumentField)
	if !hasData {
		return &GameObject{Path: collectionPath}
	}
	return p.gameObjectFrom(filePath, collectionPath, p.parseEmbedded(filePath, data))
}

func subCollectionFrom(filePath string, field textproto.Field) SubCollection {
	subCollection := SubCollection{
		ID:                field.Message.Text("id"),
		Location:          locationOfID(filePath, field.Message, field.Position),
		IDWhere:           whereTheTextOf(filePath, field.Message, "id"),
		Collection:        field.Message.Text("collection"),
		PropertyOverrides: map[string][]PropertyOverride{},
	}
	for _, instance := range field.Message.Messages("instance_properties") {
		instanceID := instance.Text("id")
		subCollection.InstancesNamed = append(subCollection.InstancesNamed, WrittenID{Name: instanceID, Location: whereTheTextOf(filePath, instance, "id")})
		for _, component := range instance.Messages("properties") {
			subCollection.PropertyOverrides[instanceID] = append(
				subCollection.PropertyOverrides[instanceID],
				propertyOverridesFrom(filePath, component)...,
			)
		}
	}
	return subCollection
}

func propertyOverridesFrom(filePath string, component *textproto.Message) []PropertyOverride {
	var overrides []PropertyOverride
	for _, property := range component.Messages("properties") {
		override := PropertyOverride{
			Component:      component.Text("id"),
			Property:       property.Text("id"),
			Type:           property.Text("type"),
			Value:          property.Text("value"),
			ComponentWhere: whereTheTextOf(filePath, component, "id"),
			NameWhere:      whereTheTextOf(filePath, property, "id"),
			ValueWhere:     whereTheTextOf(filePath, property, "value"),
		}
		if value, hasValue := property.Scalar("value"); hasValue {
			override.Location = Location{File: filePath, Position: value.Position}
		}
		if name, hasName := property.Scalar("id"); hasName {
			override.NameLocation = Location{File: filePath, Position: name.Position}
		}
		overrides = append(overrides, override)
	}
	return overrides
}
