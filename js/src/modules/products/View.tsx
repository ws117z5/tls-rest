import React, { useEffect, useState } from "react";
import axios from "axios";
import { ModuleViewProps } from "@engine/controllers/registry";
import { Field } from "@engine/fields/FormLayout";
import useT from "@engine/controllers/useT";
import Auth from "@controllers/auth";

const HDR = { headers: { "X-Request-Type": "api" } };

interface Platform {
  id: number;
  name: string;
}

interface Notice {
  ok: boolean;
  text: string;
  url?: string;
}

// Custom VIEW for "products": the product's fields plus a buy panel (quantity, payment platform, Add to cart / Buy now).
const ProductsView: React.FC<ModuleViewProps> = ({ record, navigate }) => {
  const t = useT();
  const [platforms, setPlatforms] = useState<Platform[]>([]);
  const [platform, setPlatform] = useState("");
  const [qty, setQty] = useState(1);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);
  const signedIn = Auth.isAuthenticated();
  const productId = record?.id;

  useEffect(() => {
    if (!signedIn) return;
    axios
      .get("/api/payment_platforms/available", HDR)
      .then((res) => {
        const list: Platform[] = Array.isArray(res.data) ? res.data : [];
        setPlatforms(list);
        if (list.length) setPlatform(String(list[0].id));
      })
      .catch(() => setPlatforms([]));
  }, [signedIn]);

  const errorText = (e: any) => e?.response?.data?.message || e?.response?.data?.error || e?.message || String(e);

  const addToCart = async () => {
    setBusy(true);
    setNotice(null);
    try {
      await axios.post("/api/cart/add", { product_id: productId, quantity: qty }, HDR);
      setNotice({ ok: true, text: t("Added to cart") });
    } catch (e) {
      setNotice({ ok: false, text: errorText(e) });
    } finally {
      setBusy(false);
    }
  };

  const buyNow = async () => {
    setBusy(true);
    setNotice(null);
    try {
      const res = await axios.post(`/api/products/${productId}/pay`, { platform_id: Number(platform), quantity: qty }, HDR);
      const d = res.data || {};
      if (d.redirect_url) {
        window.location.assign(d.redirect_url);
        return;
      }
      setNotice({ ok: true, text: d.message || `${t("Payment")} #${d.payment_id}: ${d.status}` });
    } catch (e) {
      setNotice({ ok: false, text: errorText(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="card my-3">
      <div className="card-body">
        <Field name="image" label="" />
        <h2 className="mb-1">{record?.name}</h2>
        <div className="mb-3" style={{ fontSize: "1.4rem", fontWeight: 600 }}>
          {record?.price} {record?.currency}
        </div>
        <div className="mb-3">
          <Field name="description" label="" />
        </div>

        {!record?.active && <div className="text-muted mb-2">{t("This product is not available.")}</div>}

        {record?.active && !signedIn && <div className="text-muted">{t("Sign in to buy.")}</div>}

        {record?.active && signedIn && (
          <div className="d-flex align-items-end gap-2 flex-wrap">
            <div>
              <div className="text-muted small text-uppercase mb-1">{t("Quantity")}</div>
              <input
                type="number"
                min={1}
                className="form-control"
                style={{ width: 90 }}
                value={qty}
                onChange={(e) => setQty(Math.max(1, Number(e.target.value) || 1))}
              />
            </div>
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
            <button type="button" className="btn btn-outline-secondary" disabled={busy} onClick={addToCart}>
              {t("Add to cart")}
            </button>
            <button type="button" className="btn btn-primary" disabled={busy || !platform} onClick={buyNow}>
              {t("Buy now")}
            </button>
            <button type="button" className="btn btn-outline-secondary" onClick={() => navigate("/cart")}>
              {t("Cart")}
            </button>
          </div>
        )}

        {notice && <div className={`small mt-2 ${notice.ok ? "text-success" : "text-danger"}`}>{notice.text}</div>}
      </div>
    </div>
  );
};

export default ProductsView;
