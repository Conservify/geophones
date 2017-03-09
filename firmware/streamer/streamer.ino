#include <Arduino.h>
#include <Wire.h>

#define GEODATA_PINX                   A5
#define GEODATA_PINY                   A4
#define GEODATA_PINZ                   A3

#define LED_PIN                        13

#define SAMPLE_RATE                    512
#define NUMBER_OF_GEODATA_SAMPLES      (SAMPLE_RATE * 2)

#define CPU_HZ                         48000000

typedef struct geodata_t {
    uint8_t pin;
    short samples[NUMBER_OF_GEODATA_SAMPLES * 2];
    short *active;
    uint32_t index;
    bool full;
} geodata_t;

geodata_t geophones[3];
uint32_t batches_written = 0;
uint32_t file_written_at = 0;
bool buffer_written = false;

void report_blink();

void sampling_start();

void sampling_take();

extern "C" char *sbrk(int32_t i);

uint32_t atsamd_free_memory() {
    char stack_dummy = 0;
    return &stack_dummy - sbrk(0);
}

#if defined(ARDUINO_SAMD_FEATHER_M0)

#define TIMER_PRESCALER_DIV            1024

void atsamd_start_timer(int frequencyHz) {
    REG_GCLK_CLKCTRL = (uint16_t)(GCLK_CLKCTRL_CLKEN | GCLK_CLKCTRL_GEN_GCLK0 | GCLK_CLKCTRL_ID(GCM_TCC2_TC3));
    while (GCLK->STATUS.bit.SYNCBUSY == 1);

    TcCount16 *TC = (TcCount16 *)TC3;

    TC->CTRLA.reg &= ~TC_CTRLA_ENABLE;

    // Use the 16-bit timer
    TC->CTRLA.reg |= TC_CTRLA_MODE_COUNT16;
    while (TC->STATUS.bit.SYNCBUSY == 1);

    // Use match mode so that the timer counter resets when the count matches the compare register
    TC->CTRLA.reg |= TC_CTRLA_WAVEGEN_MFRQ;
    while (TC->STATUS.bit.SYNCBUSY == 1);

    // Set prescaler to 1024
    TC->CTRLA.reg |= TC_CTRLA_PRESCALER_DIV1024;
    while (TC->STATUS.bit.SYNCBUSY == 1);

    // Make sure the count is in a proportional position to where it was
    // to prevent any jitter or disconnect when changing the compare value.
    int32_t cv = (CPU_HZ / (TIMER_PRESCALER_DIV * frequencyHz)) - 1;

    TC->COUNT.reg = 0;
    TC->CC[0].reg = cv;
    while (TC->STATUS.bit.SYNCBUSY == 1);

    // Enable the compare interrupt
    TC->INTENSET.reg = 0;
    TC->INTENSET.bit.MC0 = 1;

    TC->CTRLA.reg |= TC_CTRLA_ENABLE;
    while (TC->STATUS.bit.SYNCBUSY == 1);

    NVIC_EnableIRQ(TC3_IRQn);
    NVIC_SetPriority(TC3_IRQn, 1);
}

#endif

void sampling_start() {
    geophones[0].pin = GEODATA_PINX;
    geophones[1].pin = GEODATA_PINY;
    geophones[2].pin = GEODATA_PINZ;

    /* Prepare the buffer for sampling. */
    geophones[0].index = 0;
    geophones[0].full = false;
    geophones[1].index = 0;
    geophones[1].full = false;
    geophones[2].index = 0;
    geophones[2].full = false;

    /* Setup interrupts for the Arduino Mega. */
#if defined(ARDUINO_AVR_MEGA2560) || defined(ARDUINO_AVR_UNO) || defined(ARDUINO_AVR_DUEMILANOVE)

    // Set timer1 interrupt to sample at 512 Hz. */
    const unsigned short prescaling = 1;
    const unsigned short match_register = F_CPU / (prescaling * SAMPLE_RATE) - 1;
    cli();
    TCCR1B = (TCCR1B & ~_BV(WGM13)) | _BV(WGM12);
    TCCR1A = TCCR1A & ~(_BV(WGM11) | _BV(WGM10));
    TCCR1B = (TCCR1B & ~(_BV(CS12) | _BV(CS11))) | _BV(CS10);
    OCR1A = match_register;
    TIMSK1 |= _BV(OCIE1A);
    sei();

#elif defined(ARDUINO_SAM_DUE)

    /* Set a 12-bit resolutiong. */
    analogReadResolution(12);
    /* Disable write protect of PMC registers. */
    pmc_set_writeprotect(false);
    /* Enable the peripheral clock. */
    pmc_enable_periph_clk(TC3_IRQn);
    /* Configure the channel. */
    TC_Configure(TC1, 0, TC_CMR_WAVE | TC_CMR_WAVSEL_UP_RC | TC_CMR_TCCLKS_TIMER_CLOCK4);
    uint32_t rc = VARIANT_MCK / 128 / SAMPLE_RATE;
    /* Setup the timer. */
    TC_SetRA(TC1, 0, rc2 / 2);
    TC_SetRC(TC1, 0, rc);
    TC_Start(TC1, 0);
    TC1->TC_CHANNEL[0].TC_IER = TC_IER_CPCS;
    TC1->TC_CHANNEL[0].TC_IDR = ~TC_IER_CPCS;
    NVIC_EnableIRQ(TC3_IRQn);

#elif defined(ARDUINO_SAMD_FEATHER_M0)

    atsamd_start_timer(SAMPLE_RATE);

    analogReadResolution(12);

#else

#error Arduino board not supported by this software.

#endif
}

#if defined(ARDUINO_AVR_MEGA2560) || defined(ARDUINO_AVR_UNO) || defined(ARDUINO_AVR_DUEMILANOVE)

ISR(TIMER1_COMPA_vect) {
    sampling_take();
}

#elif defined(ARDUINO_SAM_DUE)

void TC3_Handler() {
    TC_GetStatus(TC1, 0);

    sampling_take();
}

#elif defined(ARDUINO_SAMD_FEATHER_M0)

void TC3_Handler() {
    TcCount16 *TC = (TcCount16 *)TC3;
    if (TC->INTFLAG.bit.MC0 == 1) {
        TC->INTFLAG.bit.MC0 = 1;

        sampling_take();
    }
}

#else

#error Arduino board not supported by this software.

#endif

void sampling_take() {
#if defined(ARDUINO_AVR_MEGA2560) || defined(ARDUINO_AVR_UNO) || defined(ARDUINO_AVR_DUEMILANOVE)
    const int adc_resolution = 1024;
#elif defined(ARDUINO_SAM_DUE)
    const int adc_resolution = 4096;
#elif defined(ARDUINO_SAMD_FEATHER_M0)
    const int adc_resolution = 4096;
#endif

    for (uint8_t i = 0; i < 3; ++i) {
        geodata_t *gd = &geophones[i];

        short sample = analogRead(gd->pin) - (adc_resolution >> 1);

        // Scale the sample.
        const int scale = 8192 / adc_resolution;
        sample = (short)((double)sample * scale);

        gd->samples[gd->index++] = sample;

        // Raise a semaphor if the buffer is full and tell which buffer
        // is active.
        if (gd->index == NUMBER_OF_GEODATA_SAMPLES) {
            gd->active = &gd->samples[0];
            gd->full = true;
        }
        else if (gd->index == NUMBER_OF_GEODATA_SAMPLES * 2) {
            gd->active = &gd->samples[NUMBER_OF_GEODATA_SAMPLES];
            gd->index = 0;
            gd->full = true;
        }
    }

    report_blink();
}

void report_blink() {
    static unsigned long timestamp = 0;
    static bool led_on = false;

#ifdef FLASH_ON_REPORT
    if (buffer_written == true) {
        buffer_written = false;
        timestamp = millis() + 50;
        digitalWrite(LED_PIN, HIGH);
        led_on = true;
    }
    if (led_on == true) {
        if (millis() > timestamp) {
            digitalWrite(LED_PIN, LOW);
            led_on = false;
        }
    }
#else
    if (millis() > timestamp) {
        digitalWrite(LED_PIN, led_on);
        led_on = !led_on;
        timestamp = millis() + 50;
    }
#endif
}

void setup() {
    Serial.begin(115200);

    pinMode(LED_PIN, OUTPUT);
    digitalWrite(LED_PIN, LOW);

    while (!Serial) {
        delay(100);
    }

    sampling_start();
}

void loop() {
    bool all_full = true;
    for (uint8_t i = 0; i < 3; ++i) {
        geodata_t *gd = &geophones[i];
        if (!gd->full) {
            all_full = false;
        }
    }
    if (all_full) {
        Serial.print(millis() - file_written_at);
        Serial.println(" writing...");

        short *gd0 = geophones[0].active;
        short *gd1 = geophones[1].active;
        short *gd2 = geophones[2].active;

        for (uint32_t i = 0; i < NUMBER_OF_GEODATA_SAMPLES; ++i) {
            Serial.print(gd0[i]);
            Serial.print(",");
            Serial.print(gd1[i]);
            Serial.print(",");
            Serial.print(gd2[i]);
            Serial.println();
        }

        for (uint8_t j = 0; j < 3; ++j) {
            geophones[j].full = false;
        }

        file_written_at = millis();
        batches_written += 1;
        buffer_written = true;
    }
}