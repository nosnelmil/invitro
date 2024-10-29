# Multi-Loader

A wrapper around loader to run multiple experiments at once with additional feature like validation, dry-run, log & metrics collection

## Prerequisites
As a wrapper around loader, multi-loader requires the initial cluster setup to be completed. See [vHive Loader Create a Cluster](https://github.com/vhive-serverless/invitro/blob/main/docs/loader.md#create-a-cluster)

## Configuration
### Multi-Loader Configuration
| Parameter name      | Data type          | Possible values | Default value | Description                                                |
|---------------------|--------------------|-----------------|---------------|------------------------------------------------------------|
| Experiments         | []LoaderExperiment | N/A             | N/A           | A list of loader experiments with their respective configurations. See [LoaderExperiment](#loaderexperiment) |
| BaseConfigPath      | string             | "cmd/multi_loader/base_loader_config.json" | N/A           | Path to the base configuration file                         |
| PreScript           | string             | any bash command | ""           | (Optional) A global script that runs once before all experiments |
| PostScript          | string             | any bash command | ""           | (Optional) A global script that runs once after all experiments  |
| MasterNode          | string             | "10.0.0.1"      | ""           | (Optional) The node acting as the master                    |
| AutoScalerNode      | string             | "10.0.0.1"      | ""           | (Optional) The node responsible for autoscaling             |
| ActivatorNode       | string             | "10.0.0.1"      | ""           | (Optional) The node responsible for activating services     |
| LoaderNode          | string             | "10.0.0.2"      | ""           | (Optional) The node responsible for running the loaders     |
| WorkerNodes         | []string           | ["10.0.0.3"]    | []           | (Optional) A list of worker nodes to distribute the workload|
| Metrics             | []string           | ["activator", "autoscaler", "top", "prometheus"] | []    | (Optional) List of supported metrics that the multi-loader will collate at the end of each experiment

> **_Note_**: 
> Node addresses are optional as Multi-Loader uses `kubectl` to find them. If needed, you can define addresses manually, which will override the automatic detection.

### LoaderExperiment
| Parameter name        | Data type              | Possible values               | Default value | Description                                                        |
|-----------------------|------------------------|-------------------------------|---------------|--------------------------------------------------------------------|
| Config                | map[string]interface{} | Any field in [LoaderConfiguration](https://github.com/vhive-serverless/invitro/blob/main/docs/configuration.md#loader-configuration-file-format) | N/A           | The configuration for each loader experiment which overrides configurations in baseLoaderConfig                      |
| Name                  | string                 | N/A                           | N/A           | The name of the loader experiment                                  |
| TracesDir             | string                 | N/A                           | N/A           | Directory containing the traces for the experiment                 |
| TracesFormat          | string                 | "data/traces/example_{}"      | N/A           | Format of the trace files **The format string "{}" is required&** |
| TraceValues           | []interface{}          | ["any", 0, 1.1]               | N/A           | Values of the trace files Replaces the "{}" in TraceFormat             |
| OutputDir             | string                 | any                           | data/out/{Name} | (Optional) Output directory for experiment results                 |
| Verbosity             | string                 | "info", "debug", "trace"      | "info"        | (Optional) Verbosity level for logging the experiment             |
| IatGeneration         | bool                   | true, false                   | false         | (Optional) Whether to Generate iats only and skip invocations |
| Generated             | bool                   | true, false                   | false         | (Optional) if iats were already generated         |
| PreScript             | string                 | any bash Command              | ""           | (Optional) Local script that runs this specific experiment |
| PostScript            | string                 | any bash Command              | ""           | (Optional) Local script that runs this specific experiment |

> **_Important_**: Only one of the following is required:
> 1. `TracesDir`, or
> 2. `TracesFormat` and `TraceValues`, or
> 3. `TracePath` within the `LoaderExperiment`'s `Config` field
>
> If more than one is defined, the order of precedence is as follows:  
> 1. `TracesDir`,  
> 2. `TracesFormat` and `TraceValues`,  
> 3. `TracePath`

> **_Note_**: 
> The `Config` field follows the same structure as the [LoaderConfiguration](https://github.com/vhive-serverless/invitro/blob/main/docs/configuration.md#loader-configuration-file-format). 
> Any field defined in `Config` will override the corresponding value from the configuration in `BaseConfigPath`, but only for that specific experiment. 
> For example, if `BaseConfigPath` has `ExperimentDuration` set to 5 minutes, and you define `ExperimentDuration` as 10 minutes in `Config`, that particular experiment will run for 10 minutes instead.

## Command Flags



