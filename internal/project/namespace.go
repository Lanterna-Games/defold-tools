package project

import (
	"sort"
	"strings"
)

type Namespace struct {
	Root        *Collection
	GameObjects map[string]*GameObject
	IsSpawned   bool

	prefixesThatCouldNotBeRead []string
}

func (n *Namespace) CouldHoldMoreThanWasRead(gameObjectPath string) bool {
	for _, prefix := range n.prefixesThatCouldNotBeRead {
		if strings.HasPrefix(gameObjectPath, prefix+"/") {
			return true
		}
	}
	return false
}

type Mount struct {
	Namespace  *Namespace
	Collection *Collection
	Prefix     string
}

func (n *Namespace) GameObjectPaths() []string {
	paths := make([]string, 0, len(n.GameObjects))
	for path := range n.GameObjects {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func (p *Project) Mounts() []Mount {
	if p.mountsAreBuilt {
		return p.mounts
	}
	for _, root := range p.rootCollections() {
		namespace := &Namespace{Root: root, GameObjects: map[string]*GameObject{}, IsSpawned: p.IsSpawnedByAFactory(root.Path)}
		p.mounts = append(p.mounts, p.mount(namespace, root, "", map[*Collection]bool{})...)
	}
	p.mountsAreBuilt = true
	return p.mounts
}

func (p *Project) NamespacesNamed(rootName string) []*Namespace {
	var namespaces []*Namespace
	for _, mount := range p.Mounts() {
		isTheRoot := mount.Collection == mount.Namespace.Root
		if isTheRoot && !mount.Namespace.IsSpawned && mount.Namespace.Root.Name == rootName {
			namespaces = append(namespaces, mount.Namespace)
		}
	}
	return namespaces
}

func (p *Project) rootCollections() []*Collection {
	isNested := map[string]bool{}
	for _, collection := range p.Collections {
		for _, subCollection := range collection.SubCollections {
			isNested[subCollection.Collection] = true
		}
	}
	var roots []*Collection
	for _, path := range sortedKeys(p.Collections) {
		if !isNested[path] {
			roots = append(roots, p.Collections[path])
		}
	}
	return roots
}

func (p *Project) mount(namespace *Namespace, collection *Collection, prefix string, ancestors map[*Collection]bool) []Mount {
	if ancestors[collection] {
		return nil
	}
	ancestors[collection] = true
	defer delete(ancestors, collection)

	mounts := []Mount{{Namespace: namespace, Collection: collection, Prefix: prefix}}
	for _, instance := range collection.Instances {
		namespace.GameObjects[prefix+"/"+instance.ID] = p.GameObjectOf(instance)
	}
	for _, subCollection := range collection.SubCollections {
		nestedPrefix := prefix + "/" + subCollection.ID
		if nested, isLoaded := p.Collections[subCollection.Collection]; isLoaded {
			mounts = append(mounts, p.mount(namespace, nested, nestedPrefix, ancestors)...)
		} else {
			namespace.prefixesThatCouldNotBeRead = append(namespace.prefixesThatCouldNotBeRead, nestedPrefix)
		}
	}
	return mounts
}

func (p *Project) GameObjectOf(instance Instance) *GameObject {
	if instance.EmbeddedObject != nil {
		return instance.EmbeddedObject
	}
	return p.GameObjects[instance.Prototype]
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
