import { ReactElement } from 'react'
import { Link } from 'react-router'
import { type OrderResponse, type Prescription, type PriceQuote } from '../api/types'
import useProfile from '../api/useProfile'
import { formatCents } from '../api/money'

const dlRowClasses = 'flex justify-between gap-4'
const dtClasses = 'text-sm text-slate-600'
const ddClasses = 'text-sm text-slate-900 text-right'

type ConfirmOrderProps = {
  rx: Prescription
  quote: PriceQuote
  onCancel: () => void
  onPlaced: (resp: OrderResponse) => void
}

export default function ConfirmOrder({ rx, quote, onCancel, onPlaced }: ConfirmOrderProps) {
  const profile = useProfile()

  let confirmState: ReactElement
  if (profile.isPending) {
    confirmState = <p className="text-slate-600">Fetching user profile...</p>
  } else if (profile.isError) {
    confirmState = <p className="text-sm text-red-600">Error fetching user profile</p>
  } else {
    const address = profile.data.address
    confirmState = (
      <div className="bg-white border border-slate-200 rounded-lg shadow-sm p-6 max-w-2xl mx-auto flex flex-col gap-6">
        <dl className="flex flex-col gap-3">
          <div className={dlRowClasses}>
            <dt className={dtClasses}>Medication</dt>
            <dd className={ddClasses}>{`${rx.medName} ${rx.strengthValue}${rx.strengthUnit}`}</dd>
          </div>
          <div className={dlRowClasses}>
            <dt className={dtClasses}>Quantity</dt>
            <dd className={ddClasses}>{rx.quantity}</dd>
          </div>
          <div className={dlRowClasses}>
            <dt className={dtClasses}>Pharmacy</dt>
            <dd className={ddClasses}>{quote.pharmacyName}</dd>
          </div>
          <div className={dlRowClasses}>
            <dt className={dtClasses}>Price Each</dt>
            <dd className={ddClasses}>{formatCents(quote.unitPriceCents)}</dd>
          </div>
          {address && (
            <div className={dlRowClasses}>
              <dt className={dtClasses}>Shipping Address</dt>
              <dd className={`${ddClasses} flex flex-col`}>
                    <span>{address.street1}</span>
                    {address.street2 && <span>{address.street2}</span>}
                    <span>{`${address.city}, ${address.state} ${address.zip}`}</span>
              </dd>
            </div>
          )}
        </dl>
        <div className="border-t border-slate-200 pt-4 flex justify-between items-baseline">
          <span className="text-sm text-slate-600">Total</span>
          <span className="text-lg font-semibold text-slate-900">
            {formatCents(quote.totalCents)}
          </span>
        </div>
        {!address &&
          <div role="alert" className="bg-amber-50 border border-amber-200 rounded-lg p-4 flex flex-col gap-2">
            <span className='text-sm text-amber-900'>No valid shipping address on file</span>
            <Link to='/profile' className='text-sm font-medium text-teal-600 underline hover:text-teal-700 focus:outline-hidden focus:ring-2 focus:ring-teal-600 focus:ring-offset-2'>
              Edit profile
            </Link>
          </div>
        }
        <div className="flex flex-col gap-3 sm:flex-row sm:justify-end">
          <button className="bg-slate-100 text-slate-700 text-sm font-medium px-4 py-2 rounded-lg hover:bg-slate-200 focus:outline-hidden focus:ring-2 focus:ring-teal-600 focus:ring-offset-2 disabled:bg-slate-300 disabled:text-slate-500 disabled:cursor-not-allowed"
            onClick={() => { onCancel() }}
          >
            Cancel
          </button>
          {address &&
            <button className="bg-teal-600 text-white text-sm font-medium px-4 py-2 rounded-lg hover:bg-teal-700 focus:outline-hidden focus:ring-2 focus:ring-teal-600 focus:ring-offset-2 disabled:bg-slate-300 disabled:text-slate-500 disabled:cursor-not-allowed"
            >
              Confirm
            </button>}
        </div>
      </div>
    )
  }
  return confirmState
}
