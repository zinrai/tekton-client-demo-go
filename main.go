package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Output struct {
	TaskRunName string `json:"taskRunName"`
	PodName     string `json:"podName"`
}

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

	// Create Kubernetes client
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
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

	ctx := context.Background()

	_, err = tektonClient.TektonV1().Tasks(targetNamespace).Get(ctx, task.Name, metav1.GetOptions{})
	if err != nil {
		_, err = tektonClient.TektonV1().Tasks(targetNamespace).Create(ctx, task, metav1.CreateOptions{})
		if err != nil {
			log.Fatalf("Error creating Task in namespace %s: %v", targetNamespace, err)
		}
		fmt.Println("Task created successfully.")
	} else {
		fmt.Println("Task already exists, skipping creation.")
	}

	// Create a TaskRun
	taskRun := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "hello-world-run-",
			Namespace:    targetNamespace,
		},
		Spec: v1.TaskRunSpec{
			TaskRef: &v1.TaskRef{
				Name: task.Name,
			},
		},
	}

	createdTaskRun, err := tektonClient.TektonV1().TaskRuns(targetNamespace).Create(ctx, taskRun, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Error creating TaskRun: %v", err)
	}

	fmt.Printf("TaskRun %s created successfully.\n", createdTaskRun.Name)

	// Get TaskRun name without waiting for completion
	taskRunWatcher, err := tektonClient.TektonV1().TaskRuns(targetNamespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", createdTaskRun.Name),
	})
	if err != nil {
		log.Fatalf("Error watching TaskRun: %v", err)
	}
	defer taskRunWatcher.Stop()

	// Get Pod name without waiting for completion
	podWatcher, err := kubeClient.CoreV1().Pods(targetNamespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/taskRun=%s", createdTaskRun.Name),
	})
	if err != nil {
		log.Fatalf("Error watching Pods: %v", err)
	}
	defer podWatcher.Stop()

	var output Output
	for {
		select {
		case event, ok := <-taskRunWatcher.ResultChan():
			if !ok {
				log.Println("TaskRun watcher channel closed")
				return
			}
			taskRun, ok := event.Object.(*v1.TaskRun)
			if !ok {
				log.Fatalf("Unexpected object type in watcher: %T", event.Object)
			}
			output.TaskRunName = taskRun.Name

		case event, ok := <-podWatcher.ResultChan():
			if !ok {
				log.Println("Pod watcher channel closed")
				return
			}
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				log.Fatalf("Unexpected object type in watcher: %T", event.Object)
			}
			output.PodName = pod.Name
			if output.PodName != "" {
				outputJSON, err := json.Marshal(output)
				if err != nil {
					log.Fatalf("Error marshaling output: %v", err)
				}
				fmt.Println(string(outputJSON))
				return
			}
		}
	}
}
