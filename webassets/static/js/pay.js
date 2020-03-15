// ClipboardJS

new ClipboardJS('.btn-copy');

// qrcode.js

const qrcode = document.querySelector("#qrcode")
const iaddr = document.querySelector("#iaddr").textContent
new QRCode(qrcode, iaddr);

// Minutes left timer

const minutesLeftElement = document.querySelector("#minutes-left")

const updateMinutesLeft = setInterval(() => {
    let minutesLeft = parseInt(minutesLeftElement.textContent, 10)
    if(minutesLeft == 0) {
        clearInterval(updateMinutesLeft)
        return
    }

    minutesLeft--
    minutesLeftElement.textContent = minutesLeft
}, 60000) // Update time left every minute (60000 ms)

// WebSocket connection to listen for payment status' update

const ws = new WebSocket(`ws://${window.location.host}/ws/payment/${paymentID}/status`) // TODO: Passare a wss per sicurezza.

ws.onmessage = event => {
    const newStatus = event.data

    const statuses = ["paid", "expired", "error"]
    const colors = ["text-success", "text-secondary", "text-danger"]

    const statusIndex = statuses.indexOf(newStatus)
    if(statusIndex === -1) {
        return
    }

    const statusElement = document.querySelector("#status")
    statusElement.innerHTML = newStatus
    statusElement.classList = "text-capitalize font-weight-bold"
    statusElement.classList.add(colors[statusIndex])

    document.querySelector(".toast .toast-header small").classList.add("d-none")
    clearInterval(updateMinutesLeft)

    document.querySelector(".toast").classList.add("blurred")
}
