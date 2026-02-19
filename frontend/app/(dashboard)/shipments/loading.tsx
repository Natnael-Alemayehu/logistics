import { ShipmentListSkeleton } from '@/components/shared'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'

export default function ShipmentsLoading() {
  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <Skeleton className="h-8 w-32" />
          <Skeleton className="mt-2 h-4 w-48" />
        </div>
        <Skeleton className="h-10 w-36" />
      </div>

      <Card>
        <CardHeader>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <Skeleton className="h-6 w-32" />
            <div className="flex flex-col gap-2 sm:flex-row sm:gap-4">
              <Skeleton className="h-10 w-full sm:w-[250px]" />
              <Skeleton className="h-10 w-full sm:w-[150px]" />
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <ShipmentListSkeleton />
        </CardContent>
      </Card>
    </div>
  )
}
