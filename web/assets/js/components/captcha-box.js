class CaptchaBox extends HTMLElement {
    constructor() {
        super();
        this.captchaId = null;
    }

    connectedCallback() {
        this.render();
        this.img = this.querySelector('#captchaImg');
        this.input = this.querySelector('#captchaInput');
        
        this.img.addEventListener('click', () => this.loadCaptcha());
        
        if (this.hasAttribute('auto-load')) {
            this.loadCaptcha();
        }
    }

    render() {
        this.innerHTML = `
            <div class="captcha-container">
                <label class="captcha-label">图形验证码</label>
                <div class="captcha-box">
                    <input type="text" id="captchaInput" placeholder="请输入验证码" maxlength="4" required>
                    <img id="captchaImg" src="" alt="点击刷新验证码" title="点击刷新验证码">
                </div>
            </div>
        `;
    }

    async loadCaptcha() {
        try {
            const response = await fetch('/api/auth/captcha');
            const data = await response.json();
            if (data.code === 200) {
                this.img.src = data.data.captcha_png_base64;
                this.captchaId = data.data.captcha_id;
            }
        } catch (err) {
            console.error('加载验证码失败:', err);
        }
    }

    getCaptchaId() {
        return this.captchaId;
    }

    getValue() {
        return this.input ? this.input.value : '';
    }

    clear() {
        if (this.input) {
            this.input.value = '';
        }
        this.loadCaptcha();
    }
}

customElements.define('captcha-box', CaptchaBox);
