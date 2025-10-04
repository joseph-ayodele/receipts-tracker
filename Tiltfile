docker_build('receipts-db', './db')
docker_build('receipts-tracker', '.')

k8s_yaml(['k8s/receipts-tracker-db.yaml', 'k8s/receipts-tracker-deployment.yaml', 'k8s/receipts-tracker-service.yaml'])

k8s_resource('receipts-db', port_forwards=['5432:5432'], objects=['receipts-db-volume'])
k8s_resource('receipts-tracker', port_forwards=['8080:8080'])
