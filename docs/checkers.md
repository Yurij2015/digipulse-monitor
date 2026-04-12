# Site Verification Methods (Checkers)

This document provides a technical overview of how the Go Monitor Service verifies site availability and performance across different check types.

## 1. HTTP Status Checker (`http`)

The primary check for web applications and websites. It ensures the web server is responsive and serving content correctly.

- **Implementation**: `w.checkHTTP`
- **Mechanism**: Performs an `HTTP HEAD` request to the target URL.
- **Why HEAD?**: It retrieves only the headers, not the full page body, making it extremely fast and lightweight for the monitored server.
- **Success Criteria**:
  - The request must complete within **10 seconds**.
  - The response status code must be in the range **200-399** (includes successes and redirects).
- **Failure**: Any network error, timeout, or status code $\ge$ 400 will mark the site as `down`.

## 2. SSL Certificate Checker (`ssl`)

Ensures encrypted traffic remains valid and certificates are not expired.

- **Implementation**: `w.checkSSL`
- **Mechanism**: Establishes a TLS connection to the host on port **443**.
- **Logic**:
  - Automatically handles protocol stripping (`http://`) and path exclusion to isolate the hostname.
  - Inspects the peer certificate presented during the handshake.
- **Success Criteria**:
  - A successful TLS handshake must be established within **5 seconds**.
  - The certificate must not be expired (`NotAfter` is in the future).
- **Metadata Returned**:
  - `issuer`: The Common Name of the certificate issuer.
  - `days_remaining`: Numeric count of days until expiration.
  - `expires_at`: ISO 8601 timestamp of expiration.

## 3. DNS Lookup Checker (`dns`)

Verifies that the domain name resolves correctly to IP addresses.

- **Implementation**: `w.checkDNS`
- **Mechanism**: Performs a standard system DNS lookup (`net.LookupIP`) for the isolated hostname.
- **Success Criteria**:
  - The system must resolve at least one IP address (A or AAAA record).
- **Metadata Returned**:
  - `ips`: A list of all IP addresses associated with the domain.

## 4. Port Reachability Checker (`port`)

A low-level network check to see if a specific service is running and accessible.

- **Implementation**: `w.checkPort`
- **Mechanism**: Attempts to open a TCP socket (`net.DialTimeout`) to the host on a specified port.
- **Parameters**:
  - `port`: The TCP port to check (provided in `task.Params`).
- **Default**: Defaults to port `443` if no port is specified.
- **Success Criteria**:
  - The TCP handshake must complete successfully within **5 seconds**.

---

### Comparison Summary

| Checker | Implementation | Level | Protocol |
| :--- | :--- | :--- | :--- |
| **HTTP** | `checkHTTP` | Application | HTTP/HTTPS |
| **SSL** | `checkSSL` | Security | TLS (port 443) |
| **DNS** | `checkDNS` | Infrastructure | DNS |
| **Port** | `checkPort` | Network | TCP |
