package project

import "slices"

type Owner struct {
	Namespace    *Namespace
	InstancePath string
	BasePrefix   string
	GameObject   *GameObject
}

func (o Owner) IsPlacedInACollection() bool {
	return o.Namespace != nil
}

func (o Owner) PathAddressedBy(url URL) string {
	if url.IsAbsolute() {
		return url.Path
	}
	return o.BasePrefix + "/" + url.Path
}

func (u URL) PointsAtItsOwnGameObject() bool {
	return u.Socket == "" && (u.Path == "" || u.Path == ".")
}

func (p *Project) Owners() []Owner {
	var owners []Owner
	isPlaced := map[*GameObject]bool{}
	for _, mount := range p.Mounts() {
		for _, instance := range mount.Collection.Instances {
			gameObject := p.GameObjectOf(instance)
			if gameObject == nil {
				continue
			}
			isPlaced[gameObject] = true
			owners = append(owners, Owner{
				Namespace:    mount.Namespace,
				InstancePath: mount.Prefix + "/" + instance.ID,
				BasePrefix:   mount.Prefix,
				GameObject:   gameObject,
			})
		}
	}
	for _, gameObjectPath := range p.GameObjectPaths() {
		gameObject := p.GameObjects[gameObjectPath]
		livesWhereItCannotBeFollowed := !isPlaced[gameObject] || p.IsSpawnedByAFactory(gameObjectPath)
		if livesWhereItCannotBeFollowed {
			owners = append(owners, Owner{InstancePath: gameObjectPath, GameObject: gameObject})
		}
	}
	return owners
}

func (p *Project) OwnersRunning(scriptPath string) []Owner {
	var running []Owner
	for _, owner := range p.Owners() {
		if slices.Contains(p.ScriptPathsOf(owner.GameObject), scriptPath) {
			running = append(running, owner)
		}
	}
	return running
}

func (p *Project) NamespacesAddressedBy(owner Owner, url URL) []*Namespace {
	staysInTheCollectionOfTheOwner := url.Socket == "" || (owner.IsPlacedInACollection() && url.Socket == owner.Namespace.Root.Name)
	switch {
	case staysInTheCollectionOfTheOwner && owner.IsPlacedInACollection():
		leavesForTheWorldItIsSpawnedInto := owner.Namespace.IsSpawned && url.IsAbsolute()
		if leavesForTheWorldItIsSpawnedInto {
			return nil
		}
		return []*Namespace{owner.Namespace}
	case staysInTheCollectionOfTheOwner || !url.IsAbsolute():
		return nil
	default:
		return p.NamespacesNamed(url.Socket)
	}
}

type Target struct {
	Name       string
	GameObject *GameObject
}

func (p *Project) TargetOf(owner Owner, url URL) (target Target, isKnown bool) {
	if url.PointsAtItsOwnGameObject() {
		return Target{Name: owner.InstancePath, GameObject: owner.GameObject}, owner.GameObject != nil
	}
	namespaces := p.NamespacesAddressedBy(owner, url)
	if len(namespaces) != 1 {
		return Target{}, false
	}
	targetPath := owner.PathAddressedBy(url)
	gameObject := namespaces[0].GameObjects[targetPath]
	return Target{Name: targetPath, GameObject: gameObject}, gameObject != nil
}
