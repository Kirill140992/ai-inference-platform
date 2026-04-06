# Cloudflare Edge Security Architecture



To protect the `api-go` service and the underlying Qdrant vector database from L7 DDoS attacks, brute force, and automated scraping, the platform leverages **Cloudflare Enterprise** capabilities. The following security layers are implemented at the edge:## 1. Web Application Firewall (WAF) & Geo-Fencing

We employ custom WAF rules to restrict access to sensitive endpoints (e.g., administrative APIs) based on geolocation and request patterns. This ensures that unauthorized reconnaissance attempts are blocked before they reach the Kubernetes cluster.*(Insert your WAF screenshot here)*`![WAF Custom Rule](../architecture/cloudflare-waf.png)`## 2. Advanced Rate Limiting

AI inference endpoints (`/api/v1/generate`) are computationally expensive. To prevent resource exhaustion and cost spikes from distributed botnets, we use Advanced Rate Limiting. Requests are grouped not just by IP, but by **JA3 Fingerprints** and User Agents, effectively mitigating attacks from rotating proxy pools.*(Insert your Rate Limiting screenshot here)*`![Rate Limiting](../architecture/cloudflare-ratelimit.png)

`## 3. Machine Learning Bot Management

To allow legitimate users while blocking scrapers from polling our vector database search API, we utilize Cloudflare's Bot Score. Traffic scoring below 30 (high probability of being an automated script) is served a `Managed Challenge` instead of a hard block, ensuring zero false positives for actual users.*(Insert your Bot Management screenshot here)*`![Bot Management](../architecture/cloudflare-bot.png)