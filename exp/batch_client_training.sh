
clean_env() {
    kn service list | awk '{print $1}' | xargs kn service delete 
    sleep 120 
}

for duration in 80 120 150 240 # 10 20 30 40 60 # 80 120 150 240
do
    go run cmd/loader.go --config cmd/config_client_hived_elastic.json  --overwrite_duration ${duration} # > log/hived_elastic_log_$duration.txt
    clean_env "$@"

    # go run cmd/loader.go --config cmd/config_client_hived.json  --overwrite_duration ${duration} # > log/hived_log_$duration.txt
    # clean_env "$@"

    # go run cmd/loader.go --config cmd/config_client_single.json  --overwrite_duration ${duration} # > log/single_log_$duration.txt
    # clean_env "$@"

    go run cmd/loader.go --config cmd/config_client_batch.json  --overwrite_duration ${duration} # > log/batch_log_$duration.txt
    clean_env "$@"

    # go run cmd/loader.go --config cmd/config_client_pipeline_batch_priority.json  --overwrite_duration ${duration} > log/single_log_$duration.txt
    # clean_env "$@"

    # go run cmd/loader.go --config cmd/config_client_batch_priority.json  --overwrite_duration ${duration} # > log/single_log_$duration.txt
    # clean_env "$@"

    # go run cmd/loader.go --config cmd/config_client_batch_prompt_bank.json  --overwrite_duration ${duration} # > log/batch_log_$duration.txt
    # clean_env "$@"

done 