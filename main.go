package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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

	err = waitForTaskRunCompletion(ctx, tektonClient, targetNamespace, createdTaskRun.Name)
	if err != nil {
		log.Fatalf("Error waiting for TaskRun completion: %v", err)
	}

	podName, err := getPodNameFromTaskRun(ctx, tektonClient, targetNamespace, createdTaskRun.Name)
	if err != nil {
		log.Fatalf("Error getting Pod name from TaskRun: %v", err)
	}

	fmt.Printf("Pod name: %s\n", podName)
}

func waitForTaskRunCompletion(ctx context.Context, tektonClient versioned.Interface, namespace, taskRunName string) error {
	return wait.PollImmediateInfinite(1*time.Second, func() (bool, error) {
		taskRun, err := tektonClient.TektonV1().TaskRuns(namespace).Get(ctx, taskRunName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if taskRun.IsDone() {
			return true, nil
		}

		return false, nil
	})
}

func getPodNameFromTaskRun(ctx context.Context, tektonClient versioned.Interface, namespace, taskRunName string) (string, error) {
	taskRun, err := tektonClient.TektonV1().TaskRuns(namespace).Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if len(taskRun.Status.PodName) == 0 {
		return "", fmt.Errorf("Pod name not found in TaskRun status")
	}

	return taskRun.Status.PodName, nil
}
