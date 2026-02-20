Официальный репозиторий ExternalDNS: https://github.com/kubernetes-sigs/external-dns

## Установка ExternalDNS и webhook

```
# Создаем пространство имен, в которое будет установлен ExternalDNS:
kubectl create ns external-dns

# создаем секрет, который содержит в себе реквизиты для доступа к API 
kubectl create secret generic external-dns-webhook-secrets \
  --namespace external-dns \
  --from-literal=api-token=ВАШ_ТОКЕН

# добавляем Helm-репозиторий Bitnami:
helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
helm repo update

# создаем файл external-dns-ec-values.yaml, который содержит в себе значения (values), нужные для установки ExternalDNS с помощью Helm:

# устанавливаем ExternalDNS
helm upgrade --install external-dns external-dns/external-dns \
  --version 1.19.0 \
  --namespace external-dns \
  --create-namespace \
  -f external-dns-ec-values.yaml

# проверяем, что Helm-чарт был успешно развернут:
helm -n external-dns list && kubectl -n external-dns get all

# удалить чарт
helm delete -n external-dns external-dns
```

## Пример конфигурационного файла external-dns-ec-values.yaml

```
resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 200m
    memory: 256Mi

webhook:
  enabled: true 
  securityContext:
    runAsUser: 65532
    runAsGroup: 65532
    runAsNonRoot: true

provider:
  name: webhook
  webhook:
    image:
      repository: ghcr.io/edge-center/external-dns-ec-webhook
      tag: v1.0.0
      pullPolicy: IfNotPresent
    
    # Переменные окружения для вашего провайдера
    env:
      - name: EC_API_URL
        value: "https://api.edgecenter.ru/dns"
      - name: EC_API_TOKEN
        valueFrom:
          secretKeyRef:
            name: external-dns-webhook-secrets
            key: api-token
      - name: EC_DRY_RUN
        value: "false"
      - name: EC_WEBHOOK_SERVER_ADDR
        value: ":8080"
    
    service:
      port: 8080  # для health checks

extraArgs:
  - "--webhook-provider-url=http://localhost:8080"

# фильтрует Kubernetes-ресурсы (сервисы, ингрессы), которые ExternalDNS будет обрабатывать
# Обрабатывать только те ресурсы, у которых есть аннотация:
# external-dns/edgecenter: "true"
annotationFilter: "external-dns/edgecenter in (true)"

# Фильтр доменов, которые будут управляться
domainFilters:
  - "cloud.ecnl.ru"

# Источники для мониторинга
sources:
  - service
  - ingress

# Политика синхронизации
policy: sync

# Идентификатор владельца TXT-записей
txtOwnerId: "external-dns-edgecenter"

# Уровень логирования
logLevel: debug

# Интервал синхронизации
interval: 30s

# Регистр для отслеживания своих записей — через TXT
registry: txt

# Опционально: включить метрики
metrics:
  enabled: true
```

## Основные параметры Helm-чарта ExternalDNS для настройки

# Настройки DNS провайдера
```
# Выбор провайдера. Основной параметр, определяющий, с каким DNS-провайдером будет работать ExternalDNS.
provider:
  name: aws  # Например: google, azure, webhook и др.

# Управление зонами
domainFilters: []
excludeDomains: []
# domainFilters: Список доменов, за которыми будет следить ExternalDNS
# excludeDomains: Домены, которые следует исключить из обработки


# Регистрация владения записями
registry: txt
txtOwnerId: "my-cluster-id"
txtPrefix: 
txtSuffix: 
# registry: Тип реестра (txt, aws-sd, dynamodb, noop)
# txtOwnerId: Обязательный параметр - уникальный идентификатор кластера
# txtPrefix/txtSuffix: Префикс/суффикс для TXT-записей
    

# Политика синхронизации
policy: upsert-only  # create-only, sync, upsert-only
# Определяет, как будут синхронизироваться записи: 
# create-only: Только создавать новые записи
# sync: Синхронизировать полностью (удалять записи, если они больше не нужны)
# upsert-only: Создавать и обновлять, но не удалять
     

# Настройки источников данных
sources:
  - service
  - ingress
# Определяет, какие ресурсы Kubernetes будут использоваться для создания DNS-записей: 
# service: Сервисы типа LoadBalancer или NodePort
# ingress: Ingress-ресурсы
# gateway: Gateway API ресурсы (требует дополнительной настройки)
     
# Дополнительные фильтры
# labelFilter: Фильтрация по меткам Kubernetes-ресурсов
# annotationFilter: Фильтрация по аннотациям (например: "external-dns.alpha.kubernetes.io/hostname=*.example.com")
# managedRecordTypes: Типы DNS-записей для управления
     

# Расширенные настройки
# Интервал обновления
interval: 1m
triggerLoopOnEvent: false
# interval: Как часто ExternalDNS проверяет изменения (по умолчанию 1 минута)
# triggerLoopOnEvent: Запускать ли цикл при событиях создания/обновления/удаления
```

# Для чего нужен txtOwnerId

Параметр txtOwnerId (который передается в ExternalDNS через флаг --txt-owner-id) является уникальным идентификатором вашего кластера Kubernetes, и он критически важен для правильной работы ExternalDNS. Вот зачем он нужен:
Идентификация владения DNS-записями:
ExternalDNS создает специальные TXT-записи параллельно с основными DNS-записями (A, CNAME и т.д.)
В этих TXT-записях хранится значение вашего txtOwnerId
Это позволяет ExternalDNS определять, какие записи он создал и может безопасно управлять
Безопасное управление существующими зонами:
Как указано в документации: "ExternalDNS is, by default, aware of the records it is managing, therefore it can safely manage non-empty hosted zones"
Без этого механизма ExternalDNS не смог бы отличить свои записи от записей, созданных другими системами или людьми
Предотвращение конфликтов:
Когда несколько экземпляров ExternalDNS работают с одной DNS-зоной, txtOwnerId помогает избежать конфликтов
Каждый экземпляр ExternalDNS будет управлять только теми записями, которые имеют его идентификатор

## Рекомендации к установке

```
# Всегда устанавливайте txtOwnerId - это критически важный параметр для предотвращения конфликтов записей:
--set txtOwnerId="my-cluster"

# Ограничьте домены фильтром чтобы ExternalDNS не пытался управлять всеми зонами:
--set domainFilters[0]=example.com

# Для production используйте политику sync чтобы ExternalDNS удалял записи, которые больше не нужны:
--set policy=sync

# Для кастомных решений используйте webhook провайдер с правильными переменными окружения:
--set provider.name=webhook
--set provider.webhook.image.repository=my-registry/webhook-provider
--set provider.webhook.env[0].name=API_URL
--set provider.webhook.env[0].value=https://api.example.com
```