const requirePasswordMiddleware = (handler, e) => {
    const password = prompt("Type your password to proceed")
    if(password) {
        const base64Password = btoa(password)
        const authHeader = `Password ${base64Password}`
        handler(authHeader, e)
    } else {
        e.preventDefault()
    }
}

const toggleInput = selector => {
    const element = document.querySelector(selector)
    const readOnly = element.readOnly
    if(readOnly) {
        element.readOnly = false
        element.classList = "form-control"
        element.focus()
        return true
    } else {
        element.readOnly = true
        element.classList = "form-control-plaintext"
        return false
    }
}

// Wallet View Key Editor Handler
document.querySelector("#viewkey-editor .toggle-editor-btn").addEventListener("click", function() {
    const newStatus = toggleInput("#viewkey-editor input")
    const submitBtn = document.querySelector("#viewkey-editor .submit-btn")
    if(newStatus == true) {
        this.innerHTML = '<i class="fas fa-times-circle"></i> Dismiss'
        submitBtn.classList.remove("d-none")
    } else {
        this.innerHTML = '<i class="fas fa-edit"></i> Edit'
        submitBtn.classList.add("d-none")
    }
})

const editViewKeyHandler = async (authHeader, e) => {
    e.preventDefault()
    
    const invalidFeedback = document.querySelector("#viewkey-editor .invalid-feedback")
    const input = document.querySelector("#viewkey-editor input")
    input.classList.remove("is-invalid")
    
    const payload = {
        viewKey: e.target["new-viewkey"].value,
    }
    
    try {
        const res = await fetch(`/store/${storeID}`, {
            method: "PUT",
            credentials: "include",
            headers: new Headers({
                "Content-Type": "application/json",
                "Accept": "application/json",
                "Authorization": authHeader,
            }),
            body: JSON.stringify(payload),
        })
        const json = await res.json()
        
        if(res.status === 200) {
            input.value = json.viewKey || ""
            
            const successAlert = document.querySelector("#viewkey-editor .alert.alert-success")
            successAlert.classList.remove("d-none")
        } else {
            invalidFeedback.innerHTML = json.error.message
            input.classList.add("is-invalid")
        }
    } catch(e) {
        invalidFeedback.innerHTML = "An error occured while sending the request."
        input.classList.add("is-invalid")
        console.error(e)
    }
}

document.querySelector("form#edit-viewkey").addEventListener("submit", requirePasswordMiddleware.bind(this, editViewKeyHandler))

const newStoreKeysHandler = async authHeader => {
    const resultAlert = document.querySelector("#store-keys .alert")
    resultAlert.classList.remove("alert-success", "alert-danger")
    
    const payload = {
        newStoreKeys: true,
    }
    
    try {
        const res = await fetch(`/store/${storeID}`, {
            method: "PUT",
            credentials: "include",
            headers: new Headers({
                "Content-Type": "application/json",
                "Accept": "application/json",
                "Authorization": authHeader
            }),
            body: JSON.stringify(payload),
        })
        const json = await res.json()
        
        if(res.status === 200) {
            const apiKeyText = document.querySelector("#api-key")
            apiKeyText.innerHTML = json.apiKey || apiKeyText.innerHTML
            const secretKeyText = document.querySelector("#secret-key")
            secretKeyText.innerHTML = json.secretKey || secretKeyText.innerHTML
            
            resultAlert.innerHTML = "New store keys generated successfully."
            resultAlert.classList.add("alert-success")
            resultAlert.classList.remove("d-none")
        } else {
            resultAlert.innerHTML = json.error.message
            resultAlert.classList.add("alert-danger")
            resultAlert.classList.remove("d-none")
        }
    } catch(e) {
        resultAlert.innerHTML = "An error occured while sending the request."
        resultAlert.classList.add("alert-danger")
        resultAlert.classList.remove("d-none")
        console.error(e)
    }
}

document.querySelector("#btn-new-store-keys").addEventListener("click", requirePasswordMiddleware.bind(this, newStoreKeysHandler))

// Webhook URL Editor Handler
document.querySelector("form#edit-webhook .toggle-editor-btn").addEventListener("click", function() {
    const newStatus = toggleInput("form#edit-webhook input")
    const submitBtn = document.querySelector("form#edit-webhook .submit-btn")
    if(newStatus == true) {
        this.innerHTML = '<i class="fas fa-times-circle"></i> Dismiss'
        submitBtn.classList.remove("d-none")
    } else {
        this.innerHTML = '<i class="fas fa-edit"></i> Edit'
        submitBtn.classList.add("d-none")
    }
})

const editWebhookHandler = async (authHeader, e) => {
    e.preventDefault()
    
    const invalidFeedback = document.querySelector("form#edit-webhook .invalid-feedback")
    const input = document.querySelector("form#edit-webhook input")
    input.classList.remove("is-invalid")
    
    const payload = {
        webhook: e.target["new-webhook"].value,
    }
    
    try {
        const res = await fetch(`/store/${storeID}`, {
            method: "PUT",
            credentials: "include",
            headers: new Headers({
                "Content-Type": "application/json",
                "Accept": "application/json",
                "Authorization": authHeader,
            }),
            body: JSON.stringify(payload),
        })
        const json = await res.json()
        
        if(res.status === 200) {
            input.value = json.webhook || ""
            
            const successAlert = document.querySelector("form#edit-webhook .alert.alert-success")
            successAlert.classList.remove("d-none")
        } else {
            invalidFeedback.innerHTML = json.error.message
            input.classList.add("is-invalid")
        }
    } catch(e) {
        invalidFeedback.innerHTML = "An error occured while sending the request."
        input.classList.add("is-invalid")
        console.error(e)
    }
}

document.querySelector("form#edit-webhook").addEventListener("submit", requirePasswordMiddleware.bind(this, editWebhookHandler))

const newWebhookSecretKeyHandler = async authHeader => {
    const resultAlert = document.querySelector("#webhook-secret-key-collapse .alert")
    resultAlert.classList.remove("alert-success", "alert-danger")
    
    const payload = {
        newWebhookSecretKey: true,
    }
    
    try {
        const res = await fetch(`/store/${storeID}`, {
            method: "PUT",
            credentials: "include",
            headers: new Headers({
                "Content-Type": "application/json",
                "Accept": "application/json",
                "Authorization": authHeader
            }),
            body: JSON.stringify(payload),
        })
        const json = await res.json()
        
        if(res.status === 200) {
            const webhookSecretKeyText = document.querySelector("#webhook-secret-key")
            webhookSecretKeyText.innerHTML = json.webhookSecretKey || webhookSecretKeyText.innerHTML
            
            resultAlert.innerHTML = "New Webhook Secret Key generated successfully."
            resultAlert.classList.add("alert-success")
            resultAlert.classList.remove("d-none")
        } else {
            resultAlert.innerHTML = json.error.message
            resultAlert.classList.add("alert-danger")
            resultAlert.classList.remove("d-none")
        }
    } catch(e) {
        resultAlert.innerHTML = "An error occured while sending the request."
        resultAlert.classList.add("alert-danger")
        resultAlert.classList.remove("d-none")
        console.error(e)
    }
}

document.querySelector("#btn-new-webhook-secret-key").addEventListener("click", requirePasswordMiddleware.bind(this, newWebhookSecretKeyHandler))

const removeStoreHandler = async (authHeader, e) => {
    const dangerAlert = document.querySelector("#remove-store .alert")
    
    try {
        const res = await fetch(`/store/${storeID}`, {
            method: "DELETE",
            credentials: "include",
            headers: new Headers({
                "Accept": "application/json",
                "Authorization": authHeader,
            }),
        })
        
        if(res.status === 204) {
            window.location.replace("/dashboard/stores")
        } else {
            const json = await res.json()
            dangerAlert.innerHTML = json.error.message
            dangerAlert.classList.remove("d-none")
        }
    } catch(e) {
        dangerAlert.innerHTML = "An error occured while sending the request."
        dangerAlert.classList.remove("d-none")
        console.error(e)
    }
}

document.querySelector("#remove-store button").addEventListener("click", requirePasswordMiddleware.bind(this, removeStoreHandler))
