# Multi-Loader

A wrapper around loader to run multiple experiments at once with additional features like validation, dry-run, log collection

## Prerequisites
As a wrapper around loader, multi-loader requires the initial cluster setup to be completed. See [vHive Loader to create a cluster](https://github.com/vhive-serverless/invitro/blob/main/docs/loader.md#create-a-cluster)

## Configuration
### Multi-Loader Configuration
| Parameter name      | Data type          | Possible values | Default value | Description                                                |
|---------------------|--------------------|-----------------|---------------|------------------------------------------------------------|
| Studies         | []LoaderStudy | N/A             | N/A           | A list of loader studies with their respective configurations. See [LoaderStudy](#loaderstudy) |
| BaseConfigPath      | string             | "tools/multi_loader/base_loader_config.json" | N/A           | Path to the base configuration file                         |
| PreScript           | string             | any bash command | ""           | (Optional) A global script that runs once before all experiments |
| PostScript          | string             | any bash command | ""           | (Optional) A global script that runs once after all experiments  |

### LoaderStudy
| Parameter name        | Data type              | Possible values               | Default value | Description                                                        |
|-----------------------|------------------------|-------------------------------|---------------|--------------------------------------------------------------------|
| Config                | map[string]interface{} | Any field in [LoaderConfiguration](https://github.com/vhive-serverless/invitro/blob/main/docs/configuration.md#loader-configuration-file-format) | N/A           | The configuration for each loader experiment which overrides configurations in baseLoaderConfig                      |
| Name                  | string                 | N/A                           | N/A           | The name of the loader experiment                                  |
| TracesDir             | string                 | N/A                           | N/A           | Directory containing the traces for the experiment                 |
| TracesFormat          | string                 | "data/traces/example_{}"      | N/A           | Format of the trace files **The format string "{}" is required** |
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

The multi-loader accepts the almost the same command-line flags as loader. 

> **_Note_**: These flags will subsequently be used during the execution of loader.go for **<u>every experiment</u>**. If you would like to define these flag for specific experiments only, define it in [LoaderStudy](#loaderstudy)

Available flags:

- **`--multiLoaderConfig`** *(default: `tools/multi_loader/multi_loader_config.json`)*:  
  Specifies the path to the multi-loader configuration file. This file contains settings and parameters that define how the multi-loader operates [see above](#multi-loader-configuration)

- **`--verbosity`** *(default: `info`)*:  
    Sets the logging verbosity level. You can choose from the following options:
    - `info`: Standard information messages.
    - `debug`: Detailed debugging messages.
    - `trace`: Extremely verbose logging, including detailed execution traces.

- **`--iatGeneration`** *(default: `false`)*:  
  If set to `true`, the multi-loader will generate inter-arrival times (IATs) only and skip the invocation of actual workloads. This is useful for scenarios where you want to analyze or generate IATs without executing the associated load.

- **`--generated`** *(default: `false`)*:  
  Indicates whether IATs have already been generated. If set to `true`, the multi-loader will use the existing IATs instead of generating new ones.

- **`--failFast`** *(default: `false`)*:  
  Determines whether the multi-loader should skip the study immediately after a failure. By default, the loader retries a failed experiment once with debug verbosity and skips the study only if the second attempt also fails. Setting this flag to `true` prevents the retry and skips the study after the first failure.



## Multi-loader Overall Flow

1. **Initialization**
    - Flags for configuration file path, verbosity, IAT generation, and execution mode are parsed
    - Logger is initialized based on verbosity level

3. **Experiment Execution Flow**
    - The multi-loader runner is instantiated with the provided configuration path.
    - A dry run is executed to validate the setup for all studies:
        - If any dry run fails, the execution terminates.
    - If all dry runs succeed, proceed to actual runs:
        - Global pre-scripts are executed.
        - Each experiment undergoes the following steps:
            1. **Pre-Execution Setup**
                - Experiment-specific pre-scripts are executed.
                - Necessary directories and folders are created.
                - Each sub-experiment is unpacked and prepared
            2. **Experiment Invocation**
                - The loader is executed with generated configurations and related flags
            3. **Post-Execution Steps**
                - Experiment-specific post-scripts are executed
                - Cleanup tasks are performed
  
4. **Completion**
    - Global post-scripts are executed.
    - Run Make Clean

### How the Dry Run Works

The dry run mode executes the loader with the `--dryRun` flag set to true after the unpacking of experiments defined in the multi-loader configurations.

In this mode, the loader performs the following actions:

- **Configuration Validation**: It verifies the experiment configurations without deploying any functions or generating invocations.
- **Error Handling**: If a fatal error occurs at any point, the experiment will halt immediately.

The purpose is to ensure that your configurations are correct and to identify any potential issues before actual execution.