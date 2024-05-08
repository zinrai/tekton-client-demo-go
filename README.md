# tekton-task-run-creator

This is a sample Go program that demonstrates how to register a Tekton Task and execute a TaskRun using the [Tekton Pipelines](https://pkg.go.dev/github.com/tektoncd/pipeline) package.

The program creates a Task named "random-strings" that generates random strings and prints them to the console. It then creates a TaskRun for the "random-strings" Task and outputs a JSON object containing the TaskRun name, Pod name, target namespace, and container start time.

## Tested Version

- Tekton Pipeline release v0.59.0 "Scottish Fold Sox" LTS
    - https://github.com/tektoncd/pipeline/releases/tag/v0.59.0

## Usage

Create a config.yaml file with the following content:

```bash
$ cat << EOF > config.yaml
targetNamespace: <your-target-namespace>
EOF
```

Replace `<your-target-namespace>` with the desired namespace where you want to create the Tekton Task and TaskRun.

```bash
$ go run main.go
Existing Task deleted successfully.
Task created successfully.
TaskRun random-strings-run-gsz68 created successfully.
{"targetNamespace":"default","taskRunName":"random-strings-run-gsz68","podName":"random-strings-run-gsz68-pod","podStartTime":"2024-05-08T00:48:28Z"}
```

## License

This project is licensed under the MIT License - see the [LICENSE](https://opensource.org/license/mit) for details.
