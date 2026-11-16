# 規格 01：Terraform 基礎（T1–T4）

> 先讀 [README](README.md)。這一份涵蓋不需要 image 就能建的資源。
> 範例程式碼從 `specs/_reference/terraform/` 讀取後貼給使用者，使用者自己打進 `terraform/`。
> 每一步結束都讓使用者跑 `terraform fmt` 和 `terraform validate`，再跑 `plan`。

使用者目前的 `terraform/` 只有 `main.tf`（`terraform { required_version / required_providers }`）、`backend.tf`、`.terraform.lock.hcl`。

每個步驟的完成條件都包含：使用者說得出這一步建了什麼、為什麼。

---

## T1：變數和 provider

**目的**：讓 Terraform 知道要操作哪個專案和 region。不會建立任何雲端資源。

**使用者要寫的**

1. 新增 `variables.tf`，**只寫 `project_id` 和 `region` 兩個變數**（參考檔裡其他變數是後面步驟才加的）。
2. 新增 `terraform.tfvars`，一行：`project_id = "<實際的 Project ID>"`。
3. 在 `main.tf` 最下面加 `provider "google"` 區塊。

**要教的重點**（前一位模型已講解過一次，這裡複習並確認理解）

- `variable "名稱" { }`：`type = string` 不加引號，因為是型別關鍵字。
- 有 `default` 的變數可以不給值，沒有的一定要給。
- `terraform.tfvars` 是特殊檔名，會自動讀取；它在 `.gitignore` 裡。
- `var.project_id` 是引用，不加引號。
- `required_providers`（要下載哪個）和 `provider "google"`（怎麼設定）的差別。
- 變數不需要重跑 `init`。

**執行**

```bash
terraform fmt
terraform validate
terraform plan
```

**完成條件**：`plan` 印出 `No changes. Your infrastructure matches the configuration.`

**可能遇到的問題**

| 現象 | 原因和處理 |
|---|---|
| `plan` 停下來問 `var.project_id` | `terraform.tfvars` 檔名打錯或不在 `terraform/` 裡 |
| `Backend initialization required` | 使用者在別的資料夾，或 `.terraform/` 被刪了，重跑 `init` |
| 憑證錯誤 | ADC 過期，重跑 `gcloud auth application-default login`，同意頁要勾選所有權限 |

---

## T2：Artifact Registry（第一個真的資源）

**目的**：建立存放 image 的倉庫。這是使用者第一次 `apply`，要放慢。

**使用者要寫的**

1. 新增 `artifact_registry.tf`（參考檔全部）。
2. 在 `main.tf` 加 `locals` 區塊，**只寫 `repo_url` 那一項**（`allowed_image_prefixes` 是 T5 才加）。

**要教的重點**

- `resource "類型" "名稱" { }`：兩個標籤。第一個是 provider 定義的資源類型，第二個是你在 Terraform 裡取的代號，只在設定檔內部使用，不是雲端上的名稱。雲端上的名稱是 `repository_id = "sep"`。
- 資源位址：`google_artifact_registry_repository.sep`，之後引用它和在 `plan` 輸出裡都是這個寫法。
- 巢狀區塊可以重複出現（兩個 `cleanup_policies`）。
- `locals`：設定檔內部的常數或計算值，用 `local.repo_url` 引用（單數 `local`）。和 `variable` 的差別是不能從外部給值。
- 字串插值 `"${var.region}-docker.pkg.dev/..."`。
- **隱含相依**：`repo_url` 裡引用了 `google_artifact_registry_repository.sep.repository_id`，Terraform 因此知道要先建好 repo。不需要自己排順序。
- cleanup policy 的用意：免費額度只有 0.5 GB。`KEEP` 優先於 `DELETE`，所以每個 image 名稱永遠保留最新 3 版，其餘超過 1 天的會被刪。

**執行和教學**

```bash
terraform fmt && terraform validate
terraform plan
```

帶使用者讀 `plan` 輸出：

- `+` 新增、`~` 就地修改、`-` 刪除、`-/+` 刪掉重建。
- `(known after apply)`：建立之後才知道的值。
- 最後一行 `Plan: 1 to add, 0 to change, 0 to destroy.` 是每次都要看的重點。

確認 plan 只有 1 個新增後：

```bash
terraform apply
```

- `apply` 會再顯示一次 plan，輸入 `yes` 才執行。
- 完成後 state 檔已經寫進 GCS bucket。可以讓使用者看：`gcloud storage ls gs://<PROJECT_ID>-tfstate/sep/`。
- 再跑一次 `terraform plan` 應該是 `No changes`，藉此說明「宣告式」和冪等。

**完成條件**

```bash
gcloud artifacts repositories list --location=asia-east1
```

列出 `sep`，格式 `DOCKER`。

**可能遇到的問題**

| 現象 | 原因和處理 |
|---|---|
| `older_than` 格式錯誤 | 參考檔用秒數 `"86400s"`，這是確定可用的寫法 |
| `403 ... artifactregistry.googleapis.com has not been used` | API 剛開還沒生效，等一分鐘重試 |

---

## T3：Cloud Build 的身分和原始碼 bucket

**目的**：`gcloud builds submit` 會把原始碼上傳到一個 bucket，再由 Cloud Build 用某個身分建置並 push。這一步準備那個身分和 bucket，並且只給它最小權限。

**使用者要寫的**：新增 `build.tf`（參考檔全部，5 個資源）。

**要教的重點**

- **Service Account（SA）是什麼**：給程式用的身分，不是給人用的。這個專案不替它產生金鑰，只讓 GCP 的服務「以它的身分執行」。
- `account_id = "sep-builder"` 會產生 email `sep-builder@<PROJECT_ID>.iam.gserviceaccount.com`。
- **引用其他資源的屬性**：`google_service_account.builder.member` 會得到 `serviceAccount:<email>`，`google_storage_bucket.build_source.name` 是 bucket 名稱。
- **IAM 三要素**：誰（`member`）、能做什麼（`role`）、對哪個東西（資源）。三個 IAM 資源分別綁在 bucket、repo、專案上。
- `_iam_member` 是「加一筆」，不會影響其他人的權限。（`_iam_policy` 和 `_iam_binding` 會覆蓋，這個專案不用。）
- **最小權限的設計理由**：不用預設的 Cloud Build 身分，也不給專案層級的 storage 權限，因為同專案裡有 tfstate bucket，builder 不應該讀得到。
- `lifecycle_rule`：原始碼壓縮檔 1 天後自動刪除，避免累積佔用儲存空間。
- `force_destroy = true`：`destroy` 時即使 bucket 裡有檔案也能刪。
- bucket 放 `US-CENTRAL1`：Cloud Storage 的免費額度只適用美國的幾個 region。

**執行**：`fmt`、`validate`、`plan`（預期 `5 to add`）、`apply`。

**完成條件**

```bash
gcloud iam service-accounts list --filter="email~sep-builder"
gcloud storage buckets describe gs://<PROJECT_ID>-sep-build-source --format="value(location)"
```

---

## T4：執行期的身分和權限

**目的**：Engine、Cleaner、任務容器、Scheduler 各用一個獨立身分，每個只拿到自己需要的權限。

**使用者要寫的**：新增 `iam.tf`（參考檔全部，7 個資源：4 個 SA、3 個權限）。

**身分設計**

| SA | 誰用 | 權限 | 理由 |
|---|---|---|---|
| `sep-engine` | Engine 服務 | 專案層級 `roles/run.developer`；對 `sep-task` 的 `roles/iam.serviceAccountUser` | 要建立和執行 job；建 job 時指定「用 sep-task 身分跑」需要借用權 |
| `sep-cleaner` | Cleaner job | 專案層級 `roles/run.developer` | 要列出和刪除 job |
| `sep-task` | 任務容器 | **沒有任何權限** | 任務跑的是外部傳入的 image，不能讓它碰到 GCP 的任何東西 |
| `sep-scheduler` | Cloud Scheduler | 對 `sep-cleaner` 這個 job 的 `roles/run.invoker`（寫在 T6 的 `cleaner.tf`） | 只能觸發這一個 job |

**要教的重點**

- 對應 K8s：SA 相當於 ServiceAccount，IAM 綁定相當於 RoleBinding。
- **`iam.serviceAccountUser`（actAs）**：這是防止權限擴張的機制。能建立 job 的人如果可以任意指定 job 的執行身分，就等於能借用任何身分。所以要明確授權「engine 可以借用 task」，而且只有 task。
- `google_service_account_iam_member`：這裡的 SA 是「被授權的對象（資源）」，不是「被授予者」，容易搞混。
- 為什麼 engine 的權限是專案層級：任務 job 是執行期才建立的，建立之前不存在，無法針對單一資源授權。代價是 engine 理論上也能修改同專案的其他 Cloud Run job，這是已知的取捨。
- 已確認 `roles/run.developer` 包含 `run.jobs.create`、`run.jobs.run`、`run.jobs.delete`、`run.jobs.list`、`run.operations.get`。

**執行**：`fmt`、`validate`、`plan`（預期 `7 to add`）、`apply`。

**完成條件**

```bash
gcloud iam service-accounts list --filter="email~^sep-"
```

列出 5 個：builder、engine、cleaner、task、scheduler。

```bash
gcloud projects get-iam-policy <PROJECT_ID> --flatten="bindings[].members" \
  --filter="bindings.members~sep-" --format="table(bindings.role, bindings.members)"
```

預期只有三筆：builder 的 `logging.logWriter`、engine 和 cleaner 的 `run.developer`。`sep-task` 和 `sep-scheduler` 不應該出現。

**到這裡可以讓使用者 commit 一次。** `terraform.tfvars` 不會被加進去，可以用 `git status` 讓使用者親眼確認。

---

## 這個階段結束時 `terraform/` 的樣子

```text
terraform/
├── main.tf                 # terraform{}、provider、locals（只有 repo_url）
├── backend.tf
├── variables.tf            # project_id、region
├── terraform.tfvars        # 不進 git
├── artifact_registry.tf    # 1 個資源
├── build.tf                # 5 個資源
├── iam.tf                  # 7 個資源
└── .terraform.lock.hcl
```

共 13 個資源。這個組合已經用 `plan` 驗證過（`13 to add`）。接下來是 [規格 02](02-go.md)。
