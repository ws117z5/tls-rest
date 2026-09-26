import React, { useEffect, useState } from "react";
import axios from "axios";
import { ModuleViewProps } from "@engine/controllers/registry";
import useT from "@engine/controllers/useT";

const HDR = { headers: { "X-Request-Type": "api" } };

interface Platform {
  id: number;
  name: string;
}

// Custom LIST for "cart": the user's lines with subtotals, remove, and Checkout through a chosen payment platform.
const CartList: React.FC<ModuleViewProps> = ({ data, navigate, reload, remove }) => {
  const t = useT();
  const rows: any[] = Array.isArray(data) ? data : [];
  const [platforms, setPlatforms] = useState<Platform[]>([]);
  const [platform, setPlatform] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ ok: boolean; text: string } | null>(null);

  useEffect(() => {
    axios
      .get("/api/payment_platforms/available", HDR)
      .then((res) => {
        const list: Platform[] = Array.isArray(res.data) ? res.data : [];
        setPlatforms(list);
        if (list.length) setPlatform(String(list[0].id));
      })
      .catch(() => setPlatforms([]));
  }, []);

  const total = rows.reduce((sum, r) => sum + (Number(r.subtotal) || 0), 0);

  const checkout = async () => {
    setBusy(true);
    setNotice(null);
    try {
      const res = await axios.post("/api/cart/checkout", { platform_id: Number(platform) }, HDR);
      const d = res.data || {};
      if (d.redirect_url) {
        window.location.assign(d.redirect_url);
        return;
      }
      setNotice({ ok: true, text: d.message || `${t("Payment")} #${d.payment_id}: ${d.status}` });
      reload();
    } catch (e: any) {
      setNotice({ ok: false, text: e?.response?.data?.message || e?.response?.data?.error || e?.message || String(e) });
    } finally {
      setBusy(false);
    }
  };

  if (rows.length === 0) {
    return <div className="text-muted p-3">{t("Your cart is empty.")}</div>;
  }

  return (
    <div className="card my-3">
      <div className="card-body">
        <table className="table table-sm align-middle">
          <thead>
            <tr>
              <th>{t("Product")}</th>
              <th>{t("Unit price")}</th>
              <th>{t("Quantity")}</th>
              <th>{t("Subtotal")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id}>
                <td>
                  <a href={`/products/${row.product_id}`} onClick={(e) => { e.preventDefault(); navigate(`/products/${row.product_id}`); }}>
                    {row.product_name || `#${row.product_id}`}
                  </a>
                </td>
                <td>{row.unit_price}</td>
                <td>{row.quantity}</td>
                <td>{row.subtotal}</td>
                <td className="text-end">
                  <button type="button" className="btn btn-outline-danger btn-sm" onClick={() => remove(row)}>
                    ×
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>

        <div className="d-flex align-items-end gap-2 flex-wrap">
          <div style={{ fontSize: "1.2rem", fontWeight: 600 }}>
            {t("Total")}: {total.toFixed(2)}
          </div>
          <div className="ms-auto" />
          <div>
            <div className="text-muted small text-uppercase mb-1">{t("Payment platform")}</div>
            <select className="form-control" value={platform} onChange={(e) => setPlatform(e.target.value)}>
              {platforms.length === 0 && <option value="">—</option>}
              {platforms.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </div>
          <button type="button" className="btn btn-primary" disabled={busy || !platform} onClick={checkout}>
            {t("Checkout")}
          </button>
        </div>

        {notice && <div className={`small mt-2 ${notice.ok ? "text-success" : "text-danger"}`}>{notice.text}</div>}
      </div>
    </div>
  );
};

export default CartList;
