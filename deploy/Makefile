.PHONY: create-cluster create-replicationcontroller create-service set-dns-record ip hostname

create-cluster:
	gcloud preview container clusters create xconf --num-nodes 3

create-replicationcontroller:
	gcloud preview container replicationcontrollers create --config-file xconf-controller.json

create-service:
	gcloud preview container services create --config-file xconf-service.json
	gcloud compute firewall-rules create xconf-node-5400 --allow=tcp:5400 --target-tags k8s-xconf-node
	gcloud compute firewall-rules create xconf-node-80 --allow=tcp:80 --target-tags k8s-xconf-node

set-dns-record:
	echo '{"additions":[{"kind":"dns#resourceRecordSet","name":"xconf.io.","rrdatas":["'$(shell $(MAKE) ip)'"],"ttl":600,"type":"A"}]}' | gcloud dns records --zone xconf-io edit

ip:
	@echo `gcloud compute forwarding-rules list xconf | grep xconf | cut -d ' ' -f 3`

hostport:
	@echo $(shell $(MAKE) ip)
