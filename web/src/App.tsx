import { useState, useEffect } from "react"
import { AccountSelector } from "@/components/AccountSelector"
import { OperationTable } from "@/components/OperationTable"
import { Select } from "@/components/ui/select"
import { api, Operation } from "@/api/client"

type OperationType = "all" | "transfer" | "account_update" | "account_update2"

function App() {
  const [account, setAccount] = useState("burndao.burn")
  const [opType, setOpType] = useState<OperationType>("all")
  const [operations, setOperations] = useState<Operation[]>([])
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const pageSize = 20

  const loadOperations = async () => {
    if (!account) return

    setLoading(true)
    try {
      let response
      if (opType === "transfer") {
        response = await api.getTransfers(account, page, pageSize)
      } else if (opType === "account_update" || opType === "account_update2") {
        // For account updates, we'll use the updates endpoint
        response = await api.getUpdates(account, page, pageSize)
        // Filter by specific type if needed
        if (opType !== "all") {
          response.operations = response.operations.filter(
            (op) => op.op_type === opType
          )
        }
      } else {
        response = await api.getOperations(account, undefined, page, pageSize)
      }

      setOperations(response.operations)
      setTotal(response.total)
      setHasMore(response.has_more)
    } catch (error) {
      console.error("Failed to load operations:", error)
      setOperations([])
      setTotal(0)
      setHasMore(false)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadOperations()
  }, [account, opType, page])

  const handlePageChange = (newPage: number) => {
    setPage(newPage)
    window.scrollTo({ top: 0, behavior: "smooth" })
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-4xl font-bold mb-2">SPS Fund Watcher</h1>
          <p className="text-muted-foreground">
            Monitor Steem blockchain operations for tracked accounts
          </p>
        </div>

        <div className="mb-6 flex flex-wrap items-center gap-4">
          <AccountSelector value={account} onChange={setAccount} />
          <div className="flex items-center gap-2">
            <label htmlFor="op-type-select" className="text-sm font-medium">
              Operation Type:
            </label>
            <Select
              id="op-type-select"
              value={opType}
              onChange={(e) => {
                setOpType(e.target.value as OperationType)
                setPage(1)
              }}
              className="min-w-[150px]"
            >
              <option value="all">All</option>
              <option value="transfer">Transfers</option>
              <option value="account_update">Account Updates</option>
              <option value="account_update2">Account Updates 2</option>
            </Select>
          </div>
        </div>

        <OperationTable
          operations={operations}
          loading={loading}
          onPageChange={handlePageChange}
          currentPage={page}
          hasMore={hasMore}
          total={total}
          pageSize={pageSize}
        />
      </div>
    </div>
  )
}

export default App
