#!/bin/bash 

# Copyright 2017-2018 Crunchy Data Solutions, Inc.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GCR_IMAGE_PREFIX=gcr.io/crunchy-dev-test

IMAGES=(
pgo-backrest-repo
pgo-backrest-restore
pgo-scheduler
pgo-sqlrunner
postgres-operator
pgo-apiserver
pgo-lspvc
pgo-rmdata
pgo-backrest
pgo-load
)

for image in "${IMAGES[@]}"
do
	docker tag $CO_IMAGE_PREFIX/$image:$CO_IMAGE_TAG   \
		$GCR_IMAGE_PREFIX/$image:$CO_IMAGE_TAG   
	gcloud docker -- push $GCR_IMAGE_PREFIX/$image:$CO_IMAGE_TAG   
done


