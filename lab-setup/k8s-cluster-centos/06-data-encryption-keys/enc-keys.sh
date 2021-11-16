#!/usr/bin/env bash

# generate random key for encryption
ENCRYPTION_KEY=$(head -c 32 /dev/urandom | base64)

# create encryption-config.yaml file
cat > encryption-config.yaml <<EOF
kind: EncryptionConfig
apiVersion: v1
resources:
  - resources:
      - secrets
    providers:
      - aescbc:
          keys:
            - name: key1
              secret: ${ENCRYPTION_KEY}
      - identity: {}
EOF
