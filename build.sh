#!/bin/bash

version=$1


t_it=$(cat .devctl.yaml)
echo '{"cities":["London", "Johannesburg", "Windhoek"]}' | gomplate -d
