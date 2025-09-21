docker_build('receipts-app-db', './db')
k8s_yaml(['db/postgres.yaml'])
k8s_resource('receipts-db', port_forwards=['5432:5432'], objects=['receipts-db-volume'])
