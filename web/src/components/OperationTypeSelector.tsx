import { Select } from "@/components/ui/select"

export type OperationType = "all" | "transfer" | "account_update" | "limit_order_create" | "limit_order_cancel"

interface OperationTypeSelectorProps {
  value: OperationType
  onChange: (type: OperationType) => void
}

export function OperationTypeSelector({ value, onChange }: OperationTypeSelectorProps) {
  return (
    <div className="flex items-center gap-2 flex-shrink-0">
      <label htmlFor="op-type-select" className="text-sm font-medium whitespace-nowrap">
        Operation Type:
      </label>
      <Select
        id="op-type-select"
        value={value}
        onChange={(e) => onChange(e.target.value as OperationType)}
        className="min-w-[150px] flex-1 w-full"
      >
        <option value="all">All</option>
        <option value="transfer">Transfers</option>
        <option value="account_update">Account Updates</option>
        <option value="limit_order_create">Limit Order Create</option>
        <option value="limit_order_cancel">Limit Order Cancel</option>
      </Select>
    </div>
  )
}
