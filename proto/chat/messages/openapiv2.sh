#!/bin/sh
#set -x

src=proto/chat/messages
dst=$src

## https://intoli.com/blog/exit-on-errors-in-bash-scripts/
# exit_on_error() {
#   exit_code=$1
#   last_command=${@:2}
#   if [ $exit_code -ne 0 ]; then
#     >&2 echo "[ERR]: \"${last_command}\" command failed with exit code ${exit_code}."
#     exit $exit_code
#   fi
# }

# # enable !! command completion
# set -o history -o histexpand

## ensure target dir exists
#mkdir -p $dst

#protos=$(ls $src/*.proto)
# NOTE: .tags are build within input files order, so ..
protos="\
$src/openapiv2.proto \
$src/catalog.proto \
"

#openapiv2_format=yaml
openapiv2_format=json
openapiv2_file_ext=.swagger.$openapiv2_format
openapiv2_filename=messages

#,disable_default_responses=true\
openapiv2_options="\
allow_merge=true\
,merge_file_name=$openapiv2_filename\
,openapi_naming_strategy=fqn\
,json_names_for_fields=false\
,disable_default_errors=true\
,repeated_path_param_separator=csv\
,allow_delete_body=true\
,logtostderr=true\
"

protoc -I proto \
 --openapiv2_out=$openapiv2_options:$dst \
 $protos

res=$? # last command execution
#echo $res
if [ $res -ne 0 ]; then
  >&2 echo "[ERR]: protoc: failed with exit code ${res}."
  exit $res
fi

# exit_on_error $? # !!

# # compose result swagger spec
# jq -s 'reduce .[] as $item ({}; . * $item)' \
#  $src/upload$openapiv2_file_ext \
#  $dst/$openapiv2_filename$openapiv2_file_ext \
#  > $dst/$openapiv2_filename-dev$openapiv2_file_ext

# mv $dst/$openapiv2_filename-dev$openapiv2_file_ext $dst/$openapiv2_filename$openapiv2_file_ext

