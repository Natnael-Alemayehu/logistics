'use client'

import { Card, CardContent } from '@/components/ui/card'
import { Construction } from 'lucide-react'

export default function ReportsPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Reports & Analytics</h1>
        <p className="text-muted-foreground">
          Overview of logistics operations performance
        </p>
      </div>

      <Card className="mt-8">
        <CardContent className="flex flex-col items-center justify-center py-24 text-center">
          <Construction className="h-16 w-16 text-muted-foreground mb-6 opacity-20" />
          <h2 className="text-2xl font-semibold mb-2">Coming Soon</h2>
          <p className="text-muted-foreground max-w-md">
            The reports and analytics module is currently under development. 
            Check back later for detailed insights into your fleet performance.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}

