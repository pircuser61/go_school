#!/bin/bash
d="{
    \"notification\": \"$1\",
    \"action\": \"$2\"
}"
echo $d

curl -k --location --request POST 'https://stage.erius.mts.ru/api/pipeliner/v1/run/320472ff-e53f-4b67-b281-6511b691a292' \
--header 'Content-Type: application/json' \
--header 'Content-Type: text/plain' \
--data "$d"