package api

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (obj *Cluster) GetObjectKind() schema.ObjectKind { return obj }
func (obj *Cluster) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	obj.APIVersion, obj.Kind = gvk.ToAPIVersionAndKind()
}
func (obj *Cluster) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(obj.APIVersion, obj.Kind)
}

var _ runtime.Object = &Cluster{}

func (obj *ClusterList) GetObjectKind() schema.ObjectKind { return obj }
func (obj *ClusterList) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	obj.APIVersion, obj.Kind = gvk.ToAPIVersionAndKind()
}
func (obj *ClusterList) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(obj.APIVersion, obj.Kind)
}

var _ runtime.Object = &ClusterList{}

func (obj *Registry) GetObjectKind() schema.ObjectKind { return obj }
func (obj *Registry) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	obj.APIVersion, obj.Kind = gvk.ToAPIVersionAndKind()
}
func (obj *Registry) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(obj.APIVersion, obj.Kind)
}

var _ runtime.Object = &Registry{}

func (obj *RegistryList) GetObjectKind() schema.ObjectKind { return obj }
func (obj *RegistryList) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	obj.APIVersion, obj.Kind = gvk.ToAPIVersionAndKind()
}
func (obj *RegistryList) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(obj.APIVersion, obj.Kind)
}

var _ runtime.Object = &RegistryList{}
