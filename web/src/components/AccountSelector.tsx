import { Select } from "@/components/ui/select"
import { api } from "@/api/client"
import { useEffect, useState } from "react"

interface AccountSelectorProps {
  value: string
  onChange: (account: string) => void
}

export function AccountSelector({ value, onChange }: AccountSelectorProps) {
  const [accounts, setAccounts] = useState<string[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api
      .getAccounts()
      .then((accs) => {
        setAccounts(accs)
        if (accs.length > 0 && !value) {
          onChange(accs[0])
        }
      })
      .catch((err) => {
        console.error("Failed to load accounts:", err)
      })
      .finally(() => {
        setLoading(false)
      })
  }, [value, onChange])

  return (
    <div className="flex items-center gap-2">
      <label htmlFor="account-select" className="text-sm font-medium">
        Account:
      </label>
      <Select
        id="account-select"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={loading}
        className="min-w-[200px]"
      >
        {accounts.map((account) => (
          <option key={account} value={account}>
            {account}
          </option>
        ))}
      </Select>
    </div>
  )
}

