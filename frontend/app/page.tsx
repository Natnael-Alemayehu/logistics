import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { 
  Truck, 
  MapPin, 
  Wifi, 
  WifiOff, 
  Smartphone, 
  Shield, 
  Clock, 
  CheckCircle2,
  ArrowRight,
  Phone,
  Mail,
  MessageSquare
} from 'lucide-react'

const features = [
  {
    icon: WifiOff,
    title: 'Offline-First Design',
    description: 'Works seamlessly in rural areas with no connectivity. Syncs automatically when connection returns.',
  },
  {
    icon: MapPin,
    title: 'Real-Time Tracking',
    description: 'Know where your trucks are without calling every driver. Last-known locations visible on the map.',
  },
  {
    icon: Shield,
    title: 'Digital Proof of Delivery',
    description: 'Capture signatures, photos, and timestamps. End delivery disputes with verifiable evidence.',
  },
  {
    icon: Smartphone,
    title: 'Simple Driver App',
    description: 'Easy-to-use Android app that drivers can learn in minutes. Works on budget smartphones.',
  },
  {
    icon: Clock,
    title: 'Save Dispatcher Time',
    description: 'Reduce driver phone calls by 40-60%. Let dispatchers focus on exceptions, not status checks.',
  },
  {
    icon: MessageSquare,
    title: 'SMS Fallback',
    description: 'Drivers without smartphones can still update status via SMS. Full coverage, no one left behind.',
  },
]

const howItWorks = [
  {
    step: 1,
    title: 'Assign Shipments',
    description: 'Create shipments in the dispatcher dashboard and assign them to drivers.',
  },
  {
    step: 2,
    title: 'Track in Real-Time',
    description: 'Monitor driver locations and shipment status from the web dashboard.',
  },
  {
    step: 3,
    title: 'Digital Delivery Proof',
    description: 'Drivers capture recipient signature and photos at delivery.',
  },
  {
    step: 4,
    title: 'Automatic Sync',
    description: 'All data syncs to the cloud when connectivity is available.',
  },
]

const stats = [
  { value: '15,000+', label: 'Trucks in Ethiopia' },
  { value: '$3.2B', label: 'Annual Freight Volume' },
  { value: '40%', label: 'Time Saved on Calls' },
  { value: '99%', label: 'Sync Success Rate' },
]

const pricingPlans = [
  {
    name: 'Starter',
    price: '$15',
    period: '/driver/month',
    description: 'Perfect for small fleets getting started',
    features: [
      'Up to 10 drivers',
      'Unlimited shipments',
      'GPS tracking',
      'Basic proof of delivery',
      'Dispatcher dashboard',
      'Email support',
    ],
    popular: false,
  },
  {
    name: 'Professional',
    price: '$20',
    period: '/driver/month',
    description: 'For growing trucking companies',
    features: [
      'Unlimited drivers',
      'All Starter features',
      'Photo proof of delivery',
      'SMS notifications (50/driver/month)',
      'Analytics dashboard',
      'Priority support',
      'API access',
    ],
    popular: true,
  },
  {
    name: 'Enterprise',
    price: 'Custom',
    period: '',
    description: 'For large fleets with custom needs',
    features: [
      'All Professional features',
      'Custom integrations',
      'Dedicated account manager',
      'On-site training',
      'SLA guarantee',
      'White-label option',
    ],
    popular: false,
  },
]

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-gradient-to-b from-slate-50 to-white dark:from-slate-950 dark:to-slate-900">
      <nav className="sticky top-0 z-50 border-b bg-white/80 backdrop-blur-md dark:bg-slate-900/80">
        <div className="container mx-auto flex h-16 items-center justify-between px-4">
          <div className="flex items-center gap-2">
            <Truck className="h-8 w-8 text-blue-600" />
            <span className="text-xl font-bold text-slate-900 dark:text-white">
              EthioTrack
            </span>
          </div>
          <div className="flex items-center gap-4">
            <Link href="/track">
              <Button variant="ghost" className="hidden sm:flex">
                Track Shipment
              </Button>
            </Link>
            <Link href="/login">
              <Button variant="outline">Sign In</Button>
            </Link>
            <Link href="/login">
              <Button>Get Started</Button>
            </Link>
          </div>
        </div>
      </nav>

      <section className="relative overflow-hidden py-20 sm:py-32">
        <div className="absolute inset-0 bg-gradient-to-r from-blue-600/10 to-purple-600/10" />
        <div className="container mx-auto px-4">
          <div className="mx-auto max-w-4xl text-center">
            <Badge variant="secondary" className="mb-6">
              Built for Ethiopian Infrastructure
            </Badge>
            <h1 className="text-4xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-6xl lg:text-7xl">
              Know Where Your{' '}
              <span className="bg-gradient-to-r from-blue-600 to-purple-600 bg-clip-text text-transparent">
                Trucks Are
              </span>
            </h1>
            <p className="mx-auto mt-6 max-w-2xl text-lg text-slate-600 dark:text-slate-400 sm:text-xl">
              The offline-first logistics tracking platform that works in Addis Ababa 
              and keeps working in rural Ethiopia. No more blind spots. No more delivery disputes.
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

      <section className="border-y bg-slate-50 py-12 dark:bg-slate-800/50">
        <div className="container mx-auto px-4">
          <div className="grid grid-cols-2 gap-8 sm:grid-cols-4">
            {stats.map((stat) => (
              <div key={stat.label} className="text-center">
                <div className="text-3xl font-bold text-blue-600 sm:text-4xl">
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

      <section id="features" className="py-20 sm:py-32">
        <div className="container mx-auto px-4">
          <div className="mx-auto max-w-3xl text-center">
            <h2 className="text-3xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
              Built for Ethiopian Conditions
            </h2>
            <p className="mt-4 text-lg text-slate-600 dark:text-slate-400">
              Global platforms require constant internet. We built for Ethiopia&apos;s reality—stable in cities, 
              unreliable on highways, non-existent in rural areas.
            </p>
          </div>
          <div className="mt-16 grid gap-8 sm:grid-cols-2 lg:grid-cols-3">
            {features.map((feature) => (
              <Card key={feature.title} className="border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-800">
                <CardContent className="p-6">
                  <div className="mb-4 inline-flex rounded-lg bg-blue-100 p-3 dark:bg-blue-900/30">
                    <feature.icon className="h-6 w-6 text-blue-600" />
                  </div>
                  <h3 className="text-xl font-semibold text-slate-900 dark:text-white">
                    {feature.title}
                  </h3>
                  <p className="mt-2 text-slate-600 dark:text-slate-400">
                    {feature.description}
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      </section>

      <section className="bg-gradient-to-r from-blue-600 to-purple-600 py-20 sm:py-32">
        <div className="container mx-auto px-4">
          <div className="mx-auto max-w-3xl text-center">
            <WifiOff className="mx-auto h-12 w-12 text-white/80" />
            <h2 className="mt-6 text-3xl font-bold tracking-tight text-white sm:text-4xl">
              Works Offline. Syncs Automatically.
            </h2>
            <p className="mt-4 text-lg text-white/90">
              Our driver app stores everything locally—GPS coordinates, delivery photos, signatures. 
              When connectivity returns, it all syncs automatically. No data loss. No manual uploads.
            </p>
            <div className="mt-10 flex flex-col items-center justify-center gap-4 sm:flex-row">
              <div className="flex items-center gap-2 rounded-full bg-white/20 px-6 py-3 text-white">
                <CheckCircle2 className="h-5 w-5" />
                <span>GPS tracking without internet</span>
              </div>
              <div className="flex items-center gap-2 rounded-full bg-white/20 px-6 py-3 text-white">
                <CheckCircle2 className="h-5 w-5" />
                <span>Signature capture offline</span>
              </div>
              <div className="flex items-center gap-2 rounded-full bg-white/20 px-6 py-3 text-white">
                <CheckCircle2 className="h-5 w-5" />
                <span>Auto-sync when connected</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section id="how-it-works" className="py-20 sm:py-32">
        <div className="container mx-auto px-4">
          <div className="mx-auto max-w-3xl text-center">
            <h2 className="text-3xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
              How It Works
            </h2>
            <p className="mt-4 text-lg text-slate-600 dark:text-slate-400">
              Get started in minutes. No complex setup required.
            </p>
          </div>
          <div className="mt-16 grid gap-8 sm:grid-cols-2 lg:grid-cols-4">
            {howItWorks.map((item) => (
              <div key={item.step} className="relative text-center">
                <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-blue-100 dark:bg-blue-900/30">
                  <span className="text-2xl font-bold text-blue-600">{item.step}</span>
                </div>
                <h3 className="text-xl font-semibold text-slate-900 dark:text-white">
                  {item.title}
                </h3>
                <p className="mt-2 text-slate-600 dark:text-slate-400">
                  {item.description}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="pricing" className="bg-slate-50 py-20 dark:bg-slate-800/50 sm:py-32">
        <div className="container mx-auto px-4">
          <div className="mx-auto max-w-3xl text-center">
            <h2 className="text-3xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
              Simple, Transparent Pricing
            </h2>
            <p className="mt-4 text-lg text-slate-600 dark:text-slate-400">
              Pay per driver. No hidden fees. Cancel anytime.
            </p>
          </div>
          <div className="mt-16 grid gap-8 lg:grid-cols-3">
            {pricingPlans.map((plan) => (
              <Card 
                key={plan.name} 
                className={`relative border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-800 ${
                  plan.popular ? 'ring-2 ring-blue-600' : ''
                }`}
              >
                {plan.popular && (
                  <div className="absolute -top-4 left-1/2 -translate-x-1/2">
                    <Badge className="bg-blue-600">Most Popular</Badge>
                  </div>
                )}
                <CardContent className="p-8">
                  <h3 className="text-xl font-semibold text-slate-900 dark:text-white">
                    {plan.name}
                  </h3>
                  <div className="mt-4 flex items-baseline">
                    <span className="text-4xl font-bold text-slate-900 dark:text-white">
                      {plan.price}
                    </span>
                    <span className="ml-1 text-slate-600 dark:text-slate-400">
                      {plan.period}
                    </span>
                  </div>
                  <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
                    {plan.description}
                  </p>
                  <ul className="mt-8 space-y-4">
                    {plan.features.map((feature) => (
                      <li key={feature} className="flex items-center gap-3">
                        <CheckCircle2 className="h-5 w-5 flex-shrink-0 text-blue-600" />
                        <span className="text-slate-600 dark:text-slate-400">{feature}</span>
                      </li>
                    ))}
                  </ul>
                  <Link href="/login" className="mt-8 block">
                    <Button 
                      className="w-full" 
                      variant={plan.popular ? 'default' : 'outline'}
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

      <section className="py-20 sm:py-32">
        <div className="container mx-auto px-4">
          <div className="mx-auto max-w-4xl">
            <div className="rounded-2xl bg-gradient-to-r from-blue-600 to-purple-600 p-8 text-center sm:p-12">
              <h2 className="text-3xl font-bold tracking-tight text-white sm:text-4xl">
                Ready to Transform Your Fleet?
              </h2>
              <p className="mt-4 text-lg text-white/90">
                Join trucking companies across Ethiopia who are already saving time and ending delivery disputes.
              </p>
              <div className="mt-8 flex flex-col items-center justify-center gap-4 sm:flex-row">
                <Link href="/login">
                  <Button size="lg" variant="secondary" className="w-full sm:w-auto">
                    Start Free Trial
                    <ArrowRight className="ml-2 h-4 w-4" />
                  </Button>
                </Link>
                <Link href="/track">
                  <Button size="lg" variant="outline" className="w-full border-white text-white hover:bg-white/10 sm:w-auto">
                    Track a Shipment
                  </Button>
                </Link>
              </div>
            </div>
          </div>
        </div>
      </section>

      <footer className="border-t bg-slate-900 py-12 text-slate-400">
        <div className="container mx-auto px-4">
          <div className="grid gap-8 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <div className="flex items-center gap-2">
                <Truck className="h-8 w-8 text-blue-500" />
                <span className="text-xl font-bold text-white">EthioTrack</span>
              </div>
              <p className="mt-4 text-sm">
                Offline-first logistics tracking platform built for Ethiopian trucking companies.
              </p>
            </div>
            <div>
              <h4 className="font-semibold text-white">Product</h4>
              <ul className="mt-4 space-y-2 text-sm">
                <li><Link href="#features" className="hover:text-white">Features</Link></li>
                <li><Link href="#pricing" className="hover:text-white">Pricing</Link></li>
                <li><Link href="/track" className="hover:text-white">Track Shipment</Link></li>
                <li><Link href="#" className="hover:text-white">Driver App</Link></li>
              </ul>
            </div>
            <div>
              <h4 className="font-semibold text-white">Company</h4>
              <ul className="mt-4 space-y-2 text-sm">
                <li><Link href="#" className="hover:text-white">About Us</Link></li>
                <li><Link href="#" className="hover:text-white">Blog</Link></li>
                <li><Link href="#" className="hover:text-white">Careers</Link></li>
                <li><Link href="#" className="hover:text-white">Contact</Link></li>
              </ul>
            </div>
            <div>
              <h4 className="font-semibold text-white">Contact</h4>
              <ul className="mt-4 space-y-3 text-sm">
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
          <div className="mt-12 border-t border-slate-800 pt-8 text-center text-sm">
            <p>&copy; {new Date().getFullYear()} EthioTrack. All rights reserved.</p>
          </div>
        </div>
      </footer>
    </div>
  )
}
