package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	kubeconfig := os.Getenv("HOME") + "/.kube/config"
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	// Create Tekton Pipeline client
	tektonClient, err := versioned.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Tekton client: %v", err)
	}

	targetNamespace := "default"

	task := &v1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: "hello-world",
		},
		Spec: v1.TaskSpec{
			Steps: []v1.Step{
				{
					Name:   "hello-world",
					Image:  "busybox",
					Script: "echo 'Hello World'",
				},
			},
		},
	}

	taskJson, err := json.Marshal(task)
	if err != nil {
		log.Fatalf("Error marshaling Task: %v", err)
	}
	fmt.Println(string(taskJson))

	ctx := context.Background()

	// Register the task in the default namespace
	_, err = tektonClient.TektonV1().Tasks(targetNamespace).Create(ctx, task, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Error creating Task in namespace %s: %v", targetNamespace, err)
	}

	fmt.Println("Task created successfully.")
}
