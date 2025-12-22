import { useState, useEffect, useRef } from "react"
import { AccountSelector } from "@/components/AccountSelector"
import { OperationTypeSelector, type OperationType } from "@/components/OperationTypeSelector"
import { OperationTable } from "@/components/OperationTable"
import { api } from "@/api/client"
import type { Operation } from "@/api/client"

function App() {
  const [account, setAccount] = useState("")
  const [opType, setOpType] = useState<OperationType>("all")
  const [operations, setOperations] = useState<Operation[]>([])
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const pageSize = 20
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const loadOperations = async () => {
    if (!account) return

    setLoading(true)
    try {
      let response
      if (opType === "transfer") {
        response = await api.getTransfers(account, page, pageSize)
      } else if (opType === "account_update") {
        // For account updates, we'll use the updates endpoint
        response = await api.getUpdates(account, page, pageSize)
        // Filter by specific type
        response.operations = response.operations.filter(
          (op) => op.op_type === opType
        )
      } else {
        response = await api.getOperations(account, opType === "all" ? undefined : opType, page, pageSize)
      }

      setOperations(response.operations || [])
      setTotal(response.total || 0)
      setHasMore(response.has_more || false)
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

  // Auto-refresh every 30 seconds when on page 1
  useEffect(() => {
    if (page === 1 && account) {
      intervalRef.current = setInterval(() => {
        loadOperations()
      }, 30000) // 30 seconds

      return () => {
        if (intervalRef.current) {
          clearInterval(intervalRef.current)
        }
      }
    } else {
      // Clear interval if not on page 1
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
    }
  }, [page, account, opType])

  const handlePageChange = (newPage: number) => {
    setPage(newPage)
    window.scrollTo({ top: 0, behavior: "smooth" })
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8 text-center">
          <h1 className="text-4xl font-bold mb-2">SPS Fund Watcher</h1>
          <p className="text-muted-foreground mb-2">
            Track specified SPS fund accounts
          </p>
          <a
            href="https://t.me/steem_fans"
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm text-primary hover:underline inline-flex items-center gap-1"
          >
            🔔 Subscribe with Telegram
          </a>
        </div>

        <div className="mb-6 flex flex-col md:flex-row md:flex-nowrap md:items-center md:justify-end gap-4 w-full">
          <AccountSelector value={account} onChange={setAccount} />
          <OperationTypeSelector
            value={opType}
            onChange={(type) => {
              setOpType(type)
              setPage(1)
            }}
          />
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

        <footer className="mt-12 pt-8 border-t text-center text-sm text-muted-foreground">
          <p>
            Created by{" "}
            <a
              href="https://steemit.com/@ety001"
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline"
            >
              @ety001
            </a>
          </p>
          <p>
            <a
              href="https://github.com/steemfans/sps-fund-watcher"
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline"
            >
              Source Code
            </a>
          </p>
        </footer>
      </div>
    </div>
  )
}

export default App
