import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Operation } from "@/api/client"
import { Button } from "@/components/ui/button"

interface OperationTableProps {
  operations: Operation[]
  loading?: boolean
  onPageChange: (page: number) => void
  currentPage: number
  hasMore: boolean
  total: number
  pageSize: number
}

export function OperationTable({
  operations,
  loading,
  onPageChange,
  currentPage,
  hasMore,
  total,
  pageSize,
}: OperationTableProps) {
  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString()
  }

  const formatOpData = (opData: Record<string, any>) => {
    const entries = Object.entries(opData)
      .filter(([key]) => key !== "memo" && key !== "json_metadata")
      .slice(0, 3)
    return entries.map(([key, value]) => `${key}: ${JSON.stringify(value)}`).join(", ")
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <div className="text-muted-foreground">Loading...</div>
      </div>
    )
  }

  if (operations.length === 0) {
    return (
      <div className="flex items-center justify-center p-8">
        <div className="text-muted-foreground">No operations found</div>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="text-sm text-muted-foreground">
        Showing {((currentPage - 1) * pageSize) + 1} to{" "}
        {Math.min(currentPage * pageSize, total)} of {total} operations
      </div>
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Block</TableHead>
              <TableHead>Time</TableHead>
              <TableHead>Account</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Details</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {operations.map((op) => (
              <TableRow key={op.id}>
                <TableCell className="font-mono text-sm">
                  {op.block_num}
                </TableCell>
                <TableCell className="text-sm">
                  {formatDate(op.timestamp)}
                </TableCell>
                <TableCell className="font-mono text-sm">
                  {op.account}
                </TableCell>
                <TableCell>
                  <span className="inline-flex items-center rounded-full bg-primary/10 px-2.5 py-0.5 text-xs font-medium text-primary">
                    {op.op_type}
                  </span>
                </TableCell>
                <TableCell className="text-sm text-muted-foreground max-w-md truncate">
                  {formatOpData(op.op_data)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <div className="flex items-center justify-between">
        <Button
          variant="outline"
          onClick={() => onPageChange(currentPage - 1)}
          disabled={currentPage === 1}
        >
          Previous
        </Button>
        <div className="text-sm text-muted-foreground">
          Page {currentPage} of {Math.ceil(total / pageSize)}
        </div>
        <Button
          variant="outline"
          onClick={() => onPageChange(currentPage + 1)}
          disabled={!hasMore}
        >
          Next
        </Button>
      </div>
    </div>
  )
}

