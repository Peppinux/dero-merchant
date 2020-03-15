#!/bin/bash

# Generates docs.html from openapi.yaml

# Requires redoc-cli: npm install -g redoc-cli

redoc-cli bundle openapi.yaml \
    --title "Documentation | Merchant | DERO" \
    -o "docs.html" \
    -t "template.hbs" \
    --options.noAutoAuth=true \
    --options.theme.colors.primary.main="#000099" \
    --options.theme.typography.fontSize="16px" \
    --options.theme.typography.code.fontSize="14px" \
    --options.expandResponses="200,201" 
