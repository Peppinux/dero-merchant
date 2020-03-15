const buildQueryString = params => {
    let queryString = "?"

    for(key in params) {
        if(params[key] !== undefined) { 
            queryString += `${key}=${params[key]}&`
        }
    }

    return (queryString.length > 1) ? queryString.slice(0, -1) : ""
}

const formatPaymentToTableRow = payment => {
    const status = {
        "pending": 0,
        "paid": 1,
        "expired": 2,
        "error": 3,
    }

    const color = [
        "table-primary", // Pending
        "table-success", // Paid
        "table-secondary", // Expired
        "table-danger", // Error
    ]

    return `
        <tr class="${color[status[payment.status]]}">
            <td>${new Date(payment.creationTime).toUTCString()}</td>
            <td>${payment.status}</td>
            <td>${payment.deroAmount}</td>
            <td>${payment.paymentID}</td>
            <td>${payment.integratedAddress}</td>
            <td>${payment.atomicDeroAmount}</td>
            <td>${payment.currency}</td>
            <td>${payment.currencyAmount}</td>
            <td>${(payment.currency === "DERO") ? `-` : `1 DERO = ${payment.exchangeRate} ${payment.currency}`}</td>
            <td>${payment.ttl} min(s)</td>
        </tr>
    `
}

const fillPaymentsTable = payments => {
    let tableRows = ""
    if(payments) {
        for(payment of payments) {
            tableRows += formatPaymentToTableRow(payment)
        }
    } else {
        tableRows = `
            <tr>
                <td>-</td>
                <td>-</td>
                <td>-</td>
                <td>-</td>
                <td>-</td>
                <td>-</td>
                <td>-</td>
                <td>-</td>
                <td>-</td>
                <td>-</td>
            </tr>
        `
    }

    document.querySelector("table tbody").innerHTML = tableRows
}

const updatePaymentsPagination = (currentPage, totalPages) => {
    const paginationList = document.querySelector("ul.pagination")    
    paginationList.innerHTML = ""

    currentPage = currentPage || 0
    
    // No pages edge case
    if(currentPage < 1) {
        return
    }

    // Previous page button
    if(currentPage > 1 && currentPage <= totalPages) {
        const previousPage = currentPage - 1
        paginationList.innerHTML += `
            <li class="page-item" data-page=${previousPage}>
                <a class="page-link" href="#" aria-label="Previous">
                    <span aria-hidden="true">&laquo;</span>
                    <span class="sr-only">Previous</span>
                </a>
            </li>
        `
    }

    const firstPage = `<li class="page-item" data-page=1><a class="page-link" href="#">1</a></li>`
    const lastPage = `<li class="page-item" data-page=${totalPages}><a class="page-link" href="#">${totalPages}</a></li>`
    
    // First page button
    paginationList.innerHTML += firstPage

    const morePages = `<li class="page-item disabled"><a class="page-link font-weight-bold" href="#">...</a></li>`

    // Current page out of bound edge case
    if(currentPage > totalPages) {
        if(totalPages != 2) {
            paginationList.innerHTML += morePages
            paginationList.innerHTML += lastPage
        }
        return
    }

    // "..." between first page and middle pages
    if((currentPage - 2) > 1) {
        paginationList.innerHTML += morePages
    }

    // Current and middle pages buttons
    for(let i = (currentPage - 2), j = (currentPage + 2); i <= j; i++) {
        if(i <= 1) {
            continue
        }
        if(i >= totalPages) {
            break
        }

        paginationList.innerHTML += `<li class="page-item" data-page=${i}><a class="page-link" href="#">${i}</a></li>`
    }

    // "..." between middle pages and last page
    if((currentPage + 2) <= (totalPages - 1)) {
        paginationList.innerHTML += morePages
    }

    // Last page button
    if(totalPages > 1) {
        paginationList.innerHTML += lastPage
    }

    // Next page button
    if(currentPage < totalPages) {
        const nextPage = currentPage + 1
        paginationList.innerHTML += `
            <li class="page-item" data-page=${nextPage}>
                <a class="page-link" href="#" aria-label="Next">
                    <span aria-hidden="true">&raquo;</span>
                    <span class="sr-only">Next</span>
                </a>
            </li>
        `
    }

    // Pages button "on click" event handler
    const pageItems = document.querySelectorAll("li.page-item")
    for(let pageItem of pageItems) {
        const pageLink = pageItem.querySelector("li.page-item a.page-link")
        pageLink.addEventListener("click", e => {
            e.preventDefault()
            loadPayments(e, pageItem.dataset.page)
            return false
        })

        if(currentPage == pageItem.dataset.page) {
            pageItem.classList.add("active")
        }
    }
}

const fetchPayments = async ({limit, page, sortBy, orderBy, status, currency} = {}) => {
    const dangerAlert = document.querySelector("div.alert")
    dangerAlert.classList.add("d-none")

    const query = buildQueryString({limit, page, sort_by: sortBy, order_by: orderBy, status, currency})

    try {
        const res = await fetch(`/store/${storeID}/payments${query}`, {
            method: "GET",
            credentials: "include",
            headers: new Headers({
                "Accept": "application/json",
            }),
        })
        const json = await res.json()

        if(res.status === 200) {
            fillPaymentsTable(json.payments)
            updatePaymentsPagination(json.page, json.totalPages)
        } else {
            fillPaymentsTable(null)
            updatePaymentsPagination(0, 0)
            dangerAlert.innerHTML = json.error.message
            dangerAlert.classList.remove("d-none")
        }
    } catch(e) {
        dangerAlert.innerHTML = "An error occured while sending the request."
        dangerAlert.classList.remove("d-none")
        console.error(e)
    }
}

const loadPayments = (e, page) => {
    if(e !== undefined) {
        e.preventDefault()
    }

    const limit = document.querySelector("#limit-form #limit")
    const sortBy = document.querySelector("#sort-by-form select option:checked")
    const status = document.querySelector("#filter-status-form select option:checked")
    const currency = document.querySelector("#filter-currency-form #currency")

    const sortAndOrder = sortBy.value.split("|")
    const [sort, order] = sortAndOrder

    const params = {
        limit: limit.value || 10,
        page: page || 1,
        sortBy: sort,
        orderBy: order,
        status: status.value,
        currency: currency.value,
    }
    
    fetchPayments(params)
}

document.querySelector("#limit-form").addEventListener("submit", e => e.preventDefault())
document.querySelector("#limit-form").addEventListener("input", loadPayments)
document.querySelector("#sort-by-form").addEventListener("change", loadPayments)
document.querySelector("#filter-status-form").addEventListener("change", loadPayments)
document.querySelector("#filter-currency-form").addEventListener("submit", e => e.preventDefault())
document.querySelector("#filter-currency-form").addEventListener("input", loadPayments)

loadPayments() // On first page load
