document.querySelector("form#change-email").addEventListener("submit", async e => {
    e.preventDefault()

    const resultAlert = document.querySelector("form#change-email div.alert")
    resultAlert.classList.value = "alert mt-3"

    const payload = {
        newEmail: e.target["new-email"].value,
        password: e.target["password"].value,
    }

    try {
        const res = await fetch("/user", {
            method: "PUT",
            credentials: "include",
            headers: new Headers({
                "Content-Type": "application/json",
                "Accept": "application/json",
            }),
            body: JSON.stringify(payload),
        })

        if(res.status === 204) {
            resultAlert.innerHTML = "Email changed successfully. A verification link will be sent to your new address shortly."
            resultAlert.classList.add("alert-success")

            document.querySelector(".card-subtitle #email").innerHTML = payload.newEmail
        } else {
            const json = await res.json()
            resultAlert.innerHTML = json.error.message
            resultAlert.classList.add("alert-danger")
        }
    } catch(e) {
        resultAlert.innerHTML = "An error occured while sending the request."
        resultAlert.classList.add("alert-danger")
        console.error(e)
    }

    e.target["password"].value = ""
})

document.querySelector("form#change-password").addEventListener("submit", async e => {
    e.preventDefault()
    
    const resultAlert = document.querySelector("form#change-password div.alert")
    resultAlert.classList.value = "alert mt-3"

    const payload = {
        oldPassword: e.target["old-password"].value,
        newPassword: e.target["new-password"].value,
        confirmNewPassword: e.target["confirm-new-password"].value,
    }

    try {
        const res = await fetch("/user", {
            method: "PUT",
            credentials: "include",
            headers: new Headers({
                "Content-Type": "application/json",
                "Accept": "application/json",
            }),
            body: JSON.stringify(payload),
        })
        
        if(res.status === 204) {
            resultAlert.innerHTML = "Password changed successfully."
            resultAlert.classList.add("alert-success")
        } else {
            const json = await res.json()
            resultAlert.innerHTML = json.error.message
            resultAlert.classList.add("alert-danger")
        }
    } catch(e) {
        resultAlert.innerHTML = "An error occured while sending the request."
        resultAlert.classList.add("error")
        console.error(e)
    }

    e.target["old-password"].value = ""
    e.target["new-password"].value = ""
    e.target["confirm-new-password"].value = ""
})
