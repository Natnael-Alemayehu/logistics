import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { 
  Truck, 
  MapPin, 
  WifiOff, 
  Smartphone, 
  ShieldCheck, 
  Clock, 
  CheckCircle2,
  ArrowRight,
  Phone,
  Mail,
  MessageSquare,
  ChevronRight,
} from 'lucide-react'

const features = [
  {
    icon: WifiOff,
    title: 'Works Offline',
    description: 'Full functionality without internet. GPS, signatures, and photos captured locally, synced automatically when connected.',
  },
  {
    icon: MapPin,
    title: 'Real-Time Tracking',
    description: 'See driver locations on a map. Know where every truck is without calling.',
  },
  {
    icon: ShieldCheck,
    title: 'Proof of Delivery',
    description: 'Digital signatures, photos, and timestamps. Resolve disputes with verifiable evidence.',
  },
  {
    icon: Smartphone,
    title: 'Simple Driver App',
    description: 'Android app designed for drivers. Works on budget smartphones. Learns in minutes.',
  },
  {
    icon: Clock,
    title: 'Save Hours Daily',
    description: 'Reduce driver phone calls by 40-60%. Dispatchers focus on exceptions, not status checks.',
  },
  {
    icon: MessageSquare,
    title: 'SMS Fallback',
    description: 'Drivers without smartphones can update status via SMS. Full fleet coverage.',
  },
]

const howItWorks = [
  {
    step: '01',
    title: 'Create Shipments',
    description: 'Add shipments in the dispatcher dashboard and assign to drivers.',
  },
  {
    step: '02',
    title: 'Track Progress',
    description: 'Monitor locations and status updates from the web dashboard.',
  },
  {
    step: '03',
    title: 'Capture Delivery',
    description: 'Driver captures signature and photos at delivery point.',
  },
  {
    step: '04',
    title: 'Auto Sync',
    description: 'All data syncs to the cloud when connectivity is available.',
  },
]

const stats = [
  { value: '15,000+', label: 'Trucks in Ethiopia' },
  { value: '40%', label: 'Time Saved on Calls' },
  { value: '99%', label: 'Sync Success Rate' },
  { value: '24/7', label: 'Offline Capability' },
]

const pricingPlans = [
  {
    name: 'Starter',
    price: '$15',
    period: '/driver/month',
    description: 'For small fleets getting started',
    features: [
      'Up to 10 drivers',
      'Unlimited shipments',
      'GPS tracking',
      'Basic proof of delivery',
      'Dispatcher dashboard',
      'Email support',
    ],
  },
  {
    name: 'Professional',
    price: '$20',
    period: '/driver/month',
    description: 'For growing trucking companies',
    features: [
      'Unlimited drivers',
      'Photo proof of delivery',
      'SMS notifications',
      'Analytics dashboard',
      'Priority support',
      'API access',
    ],
    featured: true,
  },
  {
    name: 'Enterprise',
    price: 'Custom',
    period: '',
    description: 'For large fleets with custom needs',
    features: [
      'Custom integrations',
      'Dedicated account manager',
      'On-site training',
      'SLA guarantee',
      'White-label option',
      'Volume discounts',
    ],
  },
]

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-white dark:bg-slate-950">
      <nav className="sticky top-0 z-50 border-b border-slate-200 bg-white/80 backdrop-blur-md dark:border-slate-800 dark:bg-slate-950/80">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
          <div className="flex items-center gap-2">
            <Truck className="h-7 w-7" />
            <span className="text-xl font-bold tracking-tight">EthioTrack</span>
          </div>
          <div className="flex items-center gap-3">
            <Link href="/track" className="hidden sm:block">
              <Button variant="ghost" size="sm">
                Track Shipment
              </Button>
            </Link>
            <Link href="/login">
              <Button variant="outline" size="sm">
                Sign In
              </Button>
            </Link>
            <Link href="/login">
              <Button size="sm">
                Get Started
              </Button>
            </Link>
          </div>
        </div>
      </nav>

      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-slate-100 via-white to-white dark:from-slate-900 dark:via-slate-950 dark:to-slate-950" />
        <div className="relative mx-auto max-w-7xl px-4 py-24 sm:px-6 sm:py-32 lg:px-8 lg:py-40">
          <div className="mx-auto max-w-3xl text-center">
            <Badge variant="secondary" className="mb-6 rounded-full px-4 py-1.5 text-sm font-medium">
              Built for Ethiopian Infrastructure
            </Badge>
            <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-5xl lg:text-6xl">
              Ethiopian Logistics Tracking That Works Everywhere
            </h1>
            <p className="mx-auto mt-6 max-w-2xl text-lg leading-8 text-slate-600 dark:text-slate-400">
              Track your fleet across Ethiopia—offline or online. No more blind spots. 
              No more delivery disputes. Built for the reality of Ethiopian roads.
            </p>
            <div className="mt-10 flex flex-col items-center justify-center gap-4 sm:flex-row">
              <Link href="/login">
                <Button size="lg" className="w-full sm:w-auto">
                  Start Free Trial
                  <ArrowRight className="ml-2 h-4 w-4" />
                </Button>
              </Link>
              <Link href="/track">
                <Button size="lg" variant="outline" className="w-full sm:w-auto">
                  Track a Shipment
                </Button>
              </Link>
            </div>
          </div>
        </div>
      </section>

      <section className="border-y border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900">
        <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
          <div className="grid grid-cols-2 gap-8 lg:grid-cols-4">
            {stats.map((stat) => (
              <div key={stat.label} className="text-center">
                <div className="text-3xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
                  {stat.value}
                </div>
                <div className="mt-1 text-sm text-slate-600 dark:text-slate-400">
                  {stat.label}
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="features" className="mx-auto max-w-7xl px-4 py-24 sm:px-6 sm:py-32 lg:px-8">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="text-3xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
            Fleet Tracking Built for Ethiopia&apos;s Network Reality
          </h2>
          <p className="mt-4 text-lg text-slate-600 dark:text-slate-400">
            Global platforms require constant internet. We built for Ethiopia&apos;s reality—stable in cities, 
            unreliable on highways, non-existent in rural areas.
          </p>
        </div>
        <div className="mt-16 grid gap-8 sm:grid-cols-2 lg:grid-cols-3">
          {features.map((feature) => (
            <div 
              key={feature.title} 
              className="group rounded-2xl border border-slate-200 bg-white p-8 transition-all hover:border-slate-300 dark:border-slate-800 dark:bg-slate-900 dark:hover:border-slate-700"
            >
              <div className="mb-4 inline-flex rounded-lg bg-slate-100 p-3 dark:bg-slate-800">
                <feature.icon className="h-6 w-6" />
              </div>
              <h3 className="text-lg font-semibold text-slate-900 dark:text-white">
                {feature.title}
              </h3>
              <p className="mt-2 text-slate-600 dark:text-slate-400">
                {feature.description}
              </p>
            </div>
          ))}
        </div>
      </section>

      <section className="border-y border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900">
        <div className="mx-auto max-w-7xl px-4 py-24 sm:px-6 sm:py-32 lg:px-8">
          <div className="grid items-center gap-12 lg:grid-cols-2">
            <div>
              <h2 className="text-3xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
                Works Offline. Syncs Automatically.
              </h2>
              <p className="mt-4 text-lg text-slate-600 dark:text-slate-400">
                Our driver app stores everything locally—GPS coordinates, delivery photos, signatures. 
                When connectivity returns, it all syncs automatically. No data loss. No manual uploads.
              </p>
              <ul className="mt-8 space-y-4">
                {[
                  'GPS tracking without internet',
                  'Signature capture offline',
                  'Photo proof stored locally',
                  'Auto-sync when connected',
                ].map((item) => (
                  <li key={item} className="flex items-center gap-3">
                    <div className="flex h-5 w-5 items-center justify-center rounded-full bg-slate-900 dark:bg-white">
                      <CheckCircle2 className="h-3 w-3 text-white dark:text-slate-900" />
                    </div>
                    <span className="text-slate-700 dark:text-slate-300">{item}</span>
                  </li>
                ))}
              </ul>
              <div className="mt-8">
                <Link href="/login">
                  <Button>
                    Learn More
                    <ChevronRight className="ml-1 h-4 w-4" />
                  </Button>
                </Link>
              </div>
            </div>
            <div className="relative">
              <div className="aspect-square rounded-2xl border border-slate-200 bg-white p-8 dark:border-slate-800 dark:bg-slate-950">
                <div className="flex h-full flex-col items-center justify-center text-center">
                  <div className="rounded-full border-2 border-dashed border-slate-300 p-6 dark:border-slate-700">
                    <Smartphone className="h-16 w-16 text-slate-400" />
                  </div>
                  <p className="mt-6 font-medium text-slate-900 dark:text-white">Driver App</p>
                  <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
                    Available for Android
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section id="how-it-works" className="mx-auto max-w-7xl px-4 py-24 sm:px-6 sm:py-32 lg:px-8">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="text-3xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
            How It Works
          </h2>
          <p className="mt-4 text-lg text-slate-600 dark:text-slate-400">
            Get started in minutes. No complex setup required.
          </p>
        </div>
        <div className="mt-16 grid gap-8 sm:grid-cols-2 lg:grid-cols-4">
          {howItWorks.map((item, index) => (
            <div key={item.step} className="relative">
              {index < howItWorks.length - 1 && (
                <div className="absolute left-12 top-12 hidden h-0.5 w-full border-t border-dashed border-slate-300 dark:border-slate-700 lg:block" />
              )}
              <div className="relative flex flex-col items-center text-center">
                <div className="flex h-24 w-24 items-center justify-center rounded-full border-2 border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-950">
                  <span className="text-2xl font-bold text-slate-400">{item.step}</span>
                </div>
                <h3 className="mt-6 text-lg font-semibold text-slate-900 dark:text-white">
                  {item.title}
                </h3>
                <p className="mt-2 text-slate-600 dark:text-slate-400">
                  {item.description}
                </p>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section id="pricing" className="border-t border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900">
        <div className="mx-auto max-w-7xl px-4 py-24 sm:px-6 sm:py-32 lg:px-8">
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="text-3xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
              Simple Pricing
            </h2>
            <p className="mt-4 text-lg text-slate-600 dark:text-slate-400">
              Pay per driver. No hidden fees. Cancel anytime.
            </p>
          </div>
          <div className="mt-16 grid gap-8 lg:grid-cols-3">
            {pricingPlans.map((plan) => (
              <Card 
                key={plan.name} 
                className={`rounded-2xl border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-950 ${
                  plan.featured ? 'ring-2 ring-slate-900 dark:ring-white' : ''
                }`}
              >
                <CardContent className="p-8">
                  <div className="flex items-center justify-between">
                    <h3 className="text-lg font-semibold text-slate-900 dark:text-white">
                      {plan.name}
                    </h3>
                    {plan.featured && (
                      <Badge variant="secondary" className="rounded-full">
                        Popular
                      </Badge>
                    )}
                  </div>
                  <div className="mt-4 flex items-baseline">
                    <span className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white">
                      {plan.price}
                    </span>
                    <span className="ml-1 text-slate-600 dark:text-slate-400">
                      {plan.period}
                    </span>
                  </div>
                  <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
                    {plan.description}
                  </p>
                  <ul className="mt-8 space-y-3">
                    {plan.features.map((feature) => (
                      <li key={feature} className="flex items-center gap-3">
                        <CheckCircle2 className="h-4 w-4 flex-shrink-0 text-slate-900 dark:text-white" />
                        <span className="text-sm text-slate-600 dark:text-slate-400">{feature}</span>
                      </li>
                    ))}
                  </ul>
                  <Link href="/login" className="mt-8 block">
                    <Button 
                      className="w-full" 
                      variant={plan.featured ? 'default' : 'outline'}
                    >
                      Get Started
                    </Button>
                  </Link>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-7xl px-4 py-24 sm:px-6 sm:py-32 lg:px-8">
        <div className="mx-auto max-w-4xl text-center">
          <h2 className="text-3xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
            Ready to Transform Your Fleet?
          </h2>
          <p className="mt-4 text-lg text-slate-600 dark:text-slate-400">
            Join trucking companies across Ethiopia saving time and ending delivery disputes.
          </p>
          <div className="mt-10 flex flex-col items-center justify-center gap-4 sm:flex-row">
            <Link href="/login">
              <Button size="lg">
                Start Free Trial
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            </Link>
            <Link href="/track">
              <Button size="lg" variant="outline">
                Track a Shipment
              </Button>
            </Link>
          </div>
        </div>
      </section>

      <footer className="border-t border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900">
        <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
          <div className="grid gap-8 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <div className="flex items-center gap-2">
                <Truck className="h-6 w-6" />
                <span className="text-lg font-bold">EthioTrack</span>
              </div>
              <p className="mt-4 text-sm text-slate-500 dark:text-slate-400">
                Offline-first logistics tracking platform built for Ethiopian trucking companies.
              </p>
            </div>
            <div>
              <h4 className="text-sm font-semibold text-slate-900 dark:text-white">Product</h4>
              <ul className="mt-4 space-y-2 text-sm text-slate-500 dark:text-slate-400">
                <li><Link href="#features" className="hover:text-slate-900 dark:hover:text-white">Features</Link></li>
                <li><Link href="#pricing" className="hover:text-slate-900 dark:hover:text-white">Pricing</Link></li>
                <li><Link href="/track" className="hover:text-slate-900 dark:hover:text-white">Track Shipment</Link></li>
                <li><Link href="#" className="hover:text-slate-900 dark:hover:text-white">Driver App</Link></li>
              </ul>
            </div>
            <div>
              <h4 className="text-sm font-semibold text-slate-900 dark:text-white">Company</h4>
              <ul className="mt-4 space-y-2 text-sm text-slate-500 dark:text-slate-400">
                <li><Link href="#" className="hover:text-slate-900 dark:hover:text-white">About</Link></li>
                <li><Link href="#" className="hover:text-slate-900 dark:hover:text-white">Blog</Link></li>
                <li><Link href="#" className="hover:text-slate-900 dark:hover:text-white">Careers</Link></li>
                <li><Link href="#" className="hover:text-slate-900 dark:hover:text-white">Contact</Link></li>
              </ul>
            </div>
            <div>
              <h4 className="text-sm font-semibold text-slate-900 dark:text-white">Contact</h4>
              <ul className="mt-4 space-y-3 text-sm text-slate-500 dark:text-slate-400">
                <li className="flex items-center gap-2">
                  <Phone className="h-4 w-4" />
                  <span>+251 911 123 456</span>
                </li>
                <li className="flex items-center gap-2">
                  <Mail className="h-4 w-4" />
                  <span>info@ethiotrack.et</span>
                </li>
                <li className="flex items-center gap-2">
                  <MapPin className="h-4 w-4" />
                  <span>Addis Ababa, Ethiopia</span>
                </li>
              </ul>
            </div>
          </div>
          <div className="mt-12 border-t border-slate-200 pt-8 text-center text-sm text-slate-500 dark:border-slate-800 dark:text-slate-400">
            <p>&copy; {new Date().getFullYear()} EthioTrack. All rights reserved.</p>
          </div>
        </div>
      </footer>
    </div>
  )
}
