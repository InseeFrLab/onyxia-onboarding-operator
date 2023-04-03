package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type resourceGenerationOrFinalizerChangedPredicate struct {
	predicate.Funcs
}

// Update implements default UpdateEvent filter for validating resource version change
func (resourceGenerationOrFinalizerChangedPredicate) Update(e event.UpdateEvent) bool {
	// if e.ObjectNew.GetGeneration() == e.ObjectOld.GetGeneration() && reflect.DeepEqual(e.ObjectNew.GetFinalizers(), e.ObjectOld.GetFinalizers()) {
	// 	return false
	// }
	return true
}

type resourceKindPredicate struct {
	predicate.Funcs
}

// Update implements default UpdateEvent filter for validating resource version change
func (resourceKindPredicate) Create(e event.CreateEvent) bool {
	// fmt.Println("create", e.Object)
	// fmt.Println("create", e.Object.GetName())
	// fmt.Println("create", e.Object.GetObjectKind())
	// obj, ok := e.Object.(*onyxiav1.Workspace)
	// if ok {
	// 	fmt.Println("create", obj.Status.Conditions[len(obj.Status.Conditions)-1])
	// }
	return true
}
