package controller

import v1 "k8s.io/api/core/v1"

func FindContainerByName(pod *v1.PodSpec, name string) *v1.Container {
	for i := range pod.Containers {
		if pod.Containers[i].Name == name {
			return &pod.Containers[i]
		}
	}
	return nil
}
