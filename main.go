package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	TargetNamespace string `yaml:"targetNamespace"`
}

type Output struct {
	TargetNamespace string      `json:"targetNamespace"`
	TaskRunName     string      `json:"taskRunName"`
	PodName         string      `json:"podName"`
	PodStartTime    metav1.Time `json:"podStartTime"`
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

	configData, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	var cfg Config
	err = yaml.Unmarshal(configData, &cfg)
	if err != nil {
		log.Fatalf("Error unmarshaling config data: %v", err)
	}

	targetNamespace := cfg.TargetNamespace

	task := &v1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: "random-strings",
		},
		Spec: v1.TaskSpec{
			Steps: []v1.Step{
				{
					Name:  "random-strings",
					Image: "busybox:1.36.1-uclibc",
					Command: []string{
						"/bin/sh",
						"-c",
						fmt.Sprintf("for i in $(seq 1 %d);   do cat /dev/urandom | tr -dc 'a-zA-Z0-9' | head -c %d; echo; done", 100, 80),
					},
				},
			},
		},
	}

	ctx := context.Background()

	// Delete the existing Task if it exists
	existingTask, err := tektonClient.TektonV1().Tasks(targetNamespace).Get(ctx, task.Name, metav1.GetOptions{})
	if err == nil {
		err = tektonClient.TektonV1().Tasks(targetNamespace).Delete(ctx, existingTask.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Fatalf("Error deleting existing Task in namespace %s: %v", targetNamespace, err)
		}
		fmt.Println("Existing Task deleted successfully.")
	}

	_, err = tektonClient.TektonV1().Tasks(targetNamespace).Create(ctx, task, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Error creating Task in namespace %s: %v", targetNamespace, err)
	}
	fmt.Println("Task created successfully.")

	// Create a TaskRun
	taskRun := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "random-strings-run-",
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
	output.TargetNamespace = targetNamespace
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
			output.PodStartTime = pod.CreationTimestamp
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
